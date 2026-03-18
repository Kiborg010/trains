package heuristic

import (
	"sort"

	"trains/backend/normalized"
)

// Этот файл реализует ШАГ 3 эвристики для фиксированного минимального класса схем.
//
// Область ответственности файла:
//   - построение лёгкого planning state поверх FixedClassProblem
//   - генерация высокоуровневых вариантов извлечения целевых вагонов с сортировочных путей
//   - ранжирование этих вариантов по простому и прозрачному правилу
//   - построение чернового упорядоченного плана извлечения
//
// Важная граница дизайна:
//   - этот файл НЕ генерирует scenario_steps
//   - этот файл НЕ выполняет низкоуровневые манёвры локомотива
//   - этот файл НЕ моделирует реальное выполнение перемещений
//
// Вместо этого он отвечает на более узкий вопрос:
//   "С какого сортировочного пути и с какого конца следует извлечь следующую
//   целевую группу, если предполагается, что мешающие вагоны можно временно
//   убрать на буферный вытяжной путь?"
//
// Что здесь уже реализовано:
//   - построение planning state
//   - генерация кандидатов извлечения с обоих концов каждого сортировочного пути
//   - простая оценка и tie-breaking кандидатов
//   - черновой упорядоченный план решений по извлечению
//
// Что здесь специально пока не реализовано:
//   - точная последовательность манёвров, необходимых для выполнения извлечения
//   - логика сцепки/расцепки
//   - размещение локомотива и выбор маршрута
//   - согласование с execution/runtime состоянием

// TrackPlanningState представляет текущее представление эвристики
// о состоянии одного конкретного пути.
//
// Назначение этой структуры — объединить метаданные нормализованного пути
// с текущим порядком вагонов на нём, который эвристика использует при планировании.
//
// Значение полей:
//   - Track: нормализованное описание пути, включая его роль и вместимость
//   - Wagons: вагоны, которые эвристика считает стоящими на этом пути, упорядоченные по TrackIndex
//
// Ожидаемые инварианты:
//   - Wagons отсортированы по TrackIndex после каждого обновления состояния
//   - все вагоны из Wagons в концептуальном состоянии принадлежат Track.TrackID
//   - len(Wagons) может быть меньше Track.Capacity, но намеренно не должно быть больше
type TrackPlanningState struct {
	Track  normalized.Track
	Wagons []normalized.Wagon
}

// FixedClassPlanningState — это высокоуровневое изменяемое состояние,
// используемое заготовкой эвристики для определения порядка извлечения.
//
// Оно содержит только ту информацию, которая нужна, чтобы ответить на вопрос:
// "какую целевую группу следует извлечь следующей?",
// не переходя пока к детальным манёврам.
//
// Значение полей:
//   - TargetColor: цвет, который нужно накопить в итоговом составе
//   - RequiredTargetCount: требуемая длина состава K
//   - CurrentCollectedCount: сколько целевых вагонов уже находится на пути формирования
//   - FormationTrack: текущее состояние вытяжного пути формирования
//   - BufferTrack: текущее состояние буферного вытяжного пути
//   - SortingTracks: состояния двух сортировочных путей, которые могут поставлять целевые группы
//   - RemainingTargetWagons: целевые вагоны, которые ещё не считаются собранными
//   - CollectedTargetWagons: целевые вагоны, уже отнесённые к пути формирования
//
// Ожидаемые инварианты:
//   - CurrentCollectedCount == len(CollectedTargetWagons)
//   - RequiredTargetCount >= CurrentCollectedCount
//   - FormationTrack/BufferTrack соответствуют двум ранее выбранным lead-путям
//   - RemainingTargetWagons не содержит вагонов, уже учтённых как собранные
type FixedClassPlanningState struct {
	TargetColor           string
	RequiredTargetCount   int
	CurrentCollectedCount int
	FormationTrack        TrackPlanningState
	BufferTrack           TrackPlanningState
	SortingTracks         []TrackPlanningState
	RemainingTargetWagons []normalized.Wagon
	CollectedTargetWagons []normalized.Wagon
}

// TargetExtractionCandidate описывает один возможный вариант
// "следующего извлечения" с одного конца одного сортировочного пути.
//
// Кандидат намеренно сделан высокоуровневым.
// Он не описывает, как именно выполнить извлечение,
// а только оценивает, насколько такое извлечение правдоподобно и привлекательно
// по сравнению с альтернативами.
//
// Значение полей:
//   - SourceSortingTrackID: сортировочный путь, с которого предлагается извлечение
//   - SourceSide: конец пути, с которого предполагается подход ("start" или "end")
//   - BlockingCount: число нецелевых вагонов, которые нужно сначала убрать
//   - TargetGroupSize: размер непрерывной группы целевых вагонов, доступной с этого конца
//   - TakeCount: сколько целевых вагонов реально будет взято на пути к достижению K
//   - EstimatedCost: простая эвристическая оценка, используемая для ранжирования
//   - Feasible: можно ли выполнить это извлечение при текущей вместимости буфера
//
// Ожидаемые инварианты:
//   - если Feasible == true, то BlockingCount <= доступной буферной вместимости,
//     использованной при оценке
//   - TakeCount <= TargetGroupSize
//   - TakeCount <= числу ещё недостающих целевых вагонов на момент оценки
type TargetExtractionCandidate struct {
	SourceSortingTrackID string
	SourceSide           string
	BlockingCount        int
	TargetGroupSize      int
	TakeCount            int
	EstimatedCost        int
	Feasible             bool
}

// BuildFixedClassPlanningState преобразует уже проверенный FixedClassProblem
// в изменяемое высокоуровневое состояние, используемое эвристикой
// для определения порядка извлечения.
//
// Главная идея:
// отделить уже собранные целевые вагоны (то есть уже стоящие на пути formation)
// от тех целевых вагонов, которые ещё нужно извлекать из других путей.
//
// Предположения:
//   - problem уже успешно прошёл BuildFixedClassProblem
//   - problem.FormationTrack и problem.BufferTrack — корректные lead-пути
//   - problem.WagonsByTrack содержит физический порядок вагонов по TrackIndex
//
// Возвращаемое состояние можно безопасно изменять независимо от исходного problem,
// потому что срезы вагонов клонируются.
func BuildFixedClassPlanningState(problem FixedClassProblem, requiredTargetCount int) FixedClassPlanningState {
	state := FixedClassPlanningState{
		TargetColor:         problem.TargetColor,
		RequiredTargetCount: requiredTargetCount,
		// Путь formation и буфер стартуют с теми вагонами, которые уже стоят
		// на соответствующих lead-путях. Мы клонируем и сортируем срезы,
		// чтобы отделить planner state от исходного индексированного представления задачи.
		FormationTrack: TrackPlanningState{
			Track:  problem.FormationTrack,
			Wagons: cloneAndSortWagons(problem.WagonsByTrack[problem.FormationTrack.TrackID]),
		},
		BufferTrack: TrackPlanningState{
			Track:  problem.BufferTrack,
			Wagons: cloneAndSortWagons(problem.WagonsByTrack[problem.BufferTrack.TrackID]),
		},
		SortingTracks: make([]TrackPlanningState, 0, len(problem.SortingTracks)),
	}

	// Сортировочные пути — это единственные пути, из которых в этой упрощённой
	// заготовке planner-а извлекаются новые целевые группы.
	for _, track := range problem.SortingTracks {
		state.SortingTracks = append(state.SortingTracks, TrackPlanningState{
			Track:  track,
			Wagons: cloneAndSortWagons(problem.WagonsByTrack[track.TrackID]),
		})
	}

	// Любой целевой вагон, уже стоящий на пути formation,
	// считается собранным с точки зрения порядка извлечения.
	// Все остальные целевые вагоны считаются ещё не собранными
	// и должны быть получены с сортировочных путей.
	for _, wagon := range cloneAndSortWagons(problem.TargetWagons) {
		if wagon.TrackID == problem.FormationTrack.TrackID {
			state.CollectedTargetWagons = append(state.CollectedTargetWagons, wagon)
			state.CurrentCollectedCount++
			continue
		}
		state.RemainingTargetWagons = append(state.RemainingTargetWagons, wagon)
	}

	return state
}

// EnumerateTargetExtractionCandidates оценивает оба конца каждого сортировочного пути
// и возвращает отсортированный список кандидатов на следующее извлечение.
//
// Функция отвечает только на вопрос, насколько кандидат привлекателен
// и допустим ли он при текущей вместимости буферного пути.
// Состояние при этом не изменяется.
//
// Правило сортировки:
//  1. допустимые кандидаты раньше недопустимых
//  2. меньше BlockingCount
//  3. больше TargetGroupSize
//  4. меньше EstimatedCost
//  5. лексикографически меньший ID пути
//  6. лексикографически меньший конец пути
func EnumerateTargetExtractionCandidates(state FixedClassPlanningState) []TargetExtractionCandidate {
	// Если нужное число целевых вагонов уже собрано,
	// никакого "следующего извлечения" предлагать не нужно.
	remainingNeeded := state.RequiredTargetCount - state.CurrentCollectedCount
	if remainingNeeded <= 0 {
		return nil
	}

	// Каждый сортировочный путь оценивается с двух сторон,
	// потому что доступная целевая группа может сильно различаться
	// в зависимости от того, с какого конца подходит локомотив.
	candidates := make([]TargetExtractionCandidate, 0, len(state.SortingTracks)*2)
	for _, trackState := range state.SortingTracks {
		candidates = append(candidates,
			buildTargetExtractionCandidate(trackState, "start", state.TargetColor, remainingNeeded, availableCapacity(state.BufferTrack)),
			buildTargetExtractionCandidate(trackState, "end", state.TargetColor, remainingNeeded, availableCapacity(state.BufferTrack)),
		)
	}

	// Сам порядок сортировки одновременно задаёт текущую политику scoring
	// и служит прозрачным объяснением того, почему один вариант
	// предпочтительнее другого.
	sort.Slice(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]
		if left.Feasible != right.Feasible {
			return left.Feasible
		}
		if left.BlockingCount != right.BlockingCount {
			return left.BlockingCount < right.BlockingCount
		}
		if left.TargetGroupSize != right.TargetGroupSize {
			return left.TargetGroupSize > right.TargetGroupSize
		}
		if left.EstimatedCost != right.EstimatedCost {
			return left.EstimatedCost < right.EstimatedCost
		}
		if left.SourceSortingTrackID != right.SourceSortingTrackID {
			return left.SourceSortingTrackID < right.SourceSortingTrackID
		}
		return left.SourceSide < right.SourceSide
	})

	return candidates
}

// ChooseNextTargetExtractionCandidate возвращает первого допустимого кандидата
// из уже ранжированного списка.
//
// Вся работа по ранжированию выполняется в EnumerateTargetExtractionCandidates.
// Эта функция просто выбирает лучший допустимый вариант, если такой есть.
func ChooseNextTargetExtractionCandidate(candidates []TargetExtractionCandidate) (TargetExtractionCandidate, bool) {
	for _, candidate := range candidates {
		if candidate.Feasible {
			return candidate, true
		}
	}
	return TargetExtractionCandidate{}, false
}

// BuildOrderedExtractionPlan многократно выбирает следующий допустимый кандидат
// на извлечение и применяет это высокоуровневое решение
// к клонированному planning state.
//
// Важное ограничение:
//   - эта функция не генерирует scenario steps
//   - она возвращает только упорядоченный список решений по извлечению
//
// Цикл останавливается, когда:
//   - собрано достаточно целевых вагонов, или
//   - больше нет допустимых кандидатов, или
//   - выбранный кандидат не увеличивает число собранных целевых вагонов
//     (защитная проверка от неконсистентности planner skeleton)
func BuildOrderedExtractionPlan(initialState FixedClassPlanningState) []TargetExtractionCandidate {
	// Клонируем состояние перед симуляцией, чтобы caller мог использовать
	// исходное состояние и для других проверок, и для альтернативных экспериментов.
	state := clonePlanningState(initialState)
	plan := make([]TargetExtractionCandidate, 0)

	for state.CurrentCollectedCount < state.RequiredTargetCount {
		// На каждой итерации пересчитываем кандидатов
		// из уже обновлённого абстрактного состояния.
		candidates := EnumerateTargetExtractionCandidates(state)
		chosen, ok := ChooseNextTargetExtractionCandidate(candidates)
		if !ok {
			// По текущим упрощённым правилам продолжение плана невозможно.
			break
		}
		previousCollected := state.CurrentCollectedCount
		plan = append(plan, chosen)
		applyExtractionDecision(&state, chosen)
		// Эта защитная проверка не даёт skeleton planner-у зациклиться,
		// если в будущем какое-то изменение породит извлечение,
		// которое не продвигает сборку вперёд.
		if state.CurrentCollectedCount <= previousCollected {
			break
		}
	}

	return plan
}

// buildTargetExtractionCandidate оценивает один сортировочный путь с одного конца
// и описывает ближайшую доступную целевую группу, стоящую за возможными блокировками.
//
// Модель намеренно простая:
//   - идём от выбранного конца к середине пути
//   - считаем блокирующие вагоны до первого целевого вагона
//   - измеряем непрерывную группу целевых вагонов сразу за ними
//   - считаем кандидата допустимым только если буфер вмещает все блокировки
//
// EstimatedCost — это прозрачная эвристическая оценка, а не физическое расстояние:
//   - блокировки сильно штрафуются
//   - возможность взять больше ещё нужных целевых вагонов поощряется косвенно
func buildTargetExtractionCandidate(
	trackState TrackPlanningState,
	side string,
	targetColor string,
	remainingNeeded int,
	bufferCapacity int,
) TargetExtractionCandidate {
	candidate := TargetExtractionCandidate{
		SourceSortingTrackID: trackState.Track.TrackID,
		SourceSide:           side,
		Feasible:             false,
	}

	wagons := trackState.Wagons
	if len(wagons) == 0 {
		// Пустой путь не может дать ни одного target-вагона.
		// Мы назначаем ему очень большую стоимость,
		// чтобы он ушёл в конец сортировки, но при этом оставался кандидатом как объект.
		candidate.EstimatedCost = 1_000_000
		return candidate
	}

	// Строим логический порядок обхода с выбранного конца пути.
	// Исходный срез вагонов не меняем, а просто создаём порядок индексов.
	indexes := make([]int, 0, len(wagons))
	if side == "end" {
		for i := len(wagons) - 1; i >= 0; i-- {
			indexes = append(indexes, i)
		}
	} else {
		for i := 0; i < len(wagons); i++ {
			indexes = append(indexes, i)
		}
	}

	// Ищем первый целевой вагон, достижимый с выбранного конца.
	// Все вагоны до него считаются блокирующими и должны быть убраны в буфер.
	firstTargetPos := -1
	for pos, idx := range indexes {
		if wagons[idx].Color == targetColor {
			firstTargetPos = pos
			break
		}
	}
	if firstTargetPos < 0 {
		// С этого конца вообще не видно target-вагонов.
		candidate.EstimatedCost = 1_000_000
		return candidate
	}

	candidate.BlockingCount = firstTargetPos
	// После первого target-вагона учитывается только непрерывная группа target,
	// идущая сразу за ним. Именно она считается извлекаемой на этом этапе эвристики.
	targetGroupSize := 0
	for pos := firstTargetPos; pos < len(indexes); pos++ {
		if wagons[indexes[pos]].Color != targetColor {
			break
		}
		targetGroupSize++
	}
	candidate.TargetGroupSize = targetGroupSize
	if targetGroupSize <= 0 {
		// Это защитная ветка: в норме она не должна срабатывать,
		// потому что позиция первого target уже найдена выше.
		candidate.EstimatedCost = 1_000_000
		return candidate
	}

	// Нам нужно взять не больше вагонов, чем реально ещё не хватает до K,
	// даже если непрерывная группа target больше.
	candidate.TakeCount = targetGroupSize
	if candidate.TakeCount > remainingNeeded {
		candidate.TakeCount = remainingNeeded
	}
	// Проверка допустимости по ресурсам на этом этапе только одна:
	// буфер должен вмещать все блокирующие вагоны.
	candidate.Feasible = candidate.BlockingCount <= bufferCapacity
	// Чем меньше стоимость, тем лучше.
	// Блокировки доминируют в стоимости, потому что считается,
	// что временное переставление вагонов намного дороже,
	// чем оставить часть целевых вагонов на потом.
	candidate.EstimatedCost = candidate.BlockingCount*10 + (remainingNeeded - candidate.TakeCount)
	return candidate
}

// applyExtractionDecision изменяет planning state так,
// как будто выбранное решение по извлечению успешно выполнено
// на высоком уровне абстракции.
//
// Обновление намеренно оптимистичное:
//   - блокирующие вагоны переносятся в буфер
//   - извлечённые target-вагоны добавляются на путь formation
//   - наборы collected/remaining target обновляются
//
// Никакой низкоуровневой проверки манёвров здесь не выполняется;
// это всё ещё только skeleton для выбора порядка решений.
func applyExtractionDecision(state *FixedClassPlanningState, candidate TargetExtractionCandidate) {
	for i := range state.SortingTracks {
		if state.SortingTracks[i].Track.TrackID != candidate.SourceSortingTrackID {
			continue
		}

		source := &state.SortingTracks[i]
		// Делим исходный путь на три концептуальные части:
		//   1. блокирующие вагоны, которые надо убрать в буфер
		//   2. целевые вагоны, которые считаются собранными
		//   3. остаток, который остаётся на сортировочном пути
		blocking, targets, remainder := splitTrackForExtraction(source.Wagons, candidate.SourceSide, candidate.BlockingCount, candidate.TakeCount, state.TargetColor)
		source.Wagons = remainder
		state.BufferTrack.Wagons = append(state.BufferTrack.Wagons, blocking...)
		state.FormationTrack.Wagons = append(state.FormationTrack.Wagons, targets...)
		state.CollectedTargetWagons = append(state.CollectedTargetWagons, targets...)
		state.CurrentCollectedCount += len(targets)
		state.RemainingTargetWagons = removeCollectedTargets(state.RemainingTargetWagons, targets)

		// Пересортировываем все затронутые пути,
		// чтобы последующие шаги снова видели каноническое представление
		// порядка вагонов по TrackIndex.
		source.Wagons = cloneAndSortWagons(source.Wagons)
		state.BufferTrack.Wagons = cloneAndSortWagons(state.BufferTrack.Wagons)
		state.FormationTrack.Wagons = cloneAndSortWagons(state.FormationTrack.Wagons)
		return
	}
}

// splitTrackForExtraction делит список вагонов на пути на:
//   - блокирующие вагоны со стороны подхода
//   - целевые вагоны, которые сразу извлекаются после блокировок
//   - оставшиеся вагоны, которые остаются на исходном пути
//
// Для side == "start" используется естественный порядок TrackIndex,
// а для side == "end" — обратный.
// Для случая "end" переиспользуется логика "start" через предварительный разворот копии среза.
func splitTrackForExtraction(
	wagons []normalized.Wagon,
	side string,
	blockingCount int,
	takeCount int,
	targetColor string,
) ([]normalized.Wagon, []normalized.Wagon, []normalized.Wagon) {
	if side == "end" {
		// Переиспользуем ту же самую логику извлечения,
		// предварительно развернув порядок вагонов.
		// Это позволяет использовать одинаковые правила для обоих концов пути.
		reversed := append([]normalized.Wagon{}, wagons...)
		for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
			reversed[i], reversed[j] = reversed[j], reversed[i]
		}
		blocking, targets, remainder := splitTrackForExtraction(reversed, "start", blockingCount, takeCount, targetColor)
		return blocking, targets, remainder
	}

	// Работаем с отсортированной копией, чтобы не изменять входной срез caller-а.
	ordered := append([]normalized.Wagon{}, wagons...)
	head := 0
	blocking := make([]normalized.Wagon, 0, blockingCount)
	// Сначала забираем ровно столько блокирующих вагонов,
	// сколько было рассчитано для выбранного кандидата.
	for head < len(ordered) && len(blocking) < blockingCount {
		blocking = append(blocking, ordered[head])
		head++
	}

	targets := make([]normalized.Wagon, 0, takeCount)
	// Затем забираем до takeCount подряд идущих target-вагонов.
	// Как только встречается нецелевой вагон, извлекаемая группа заканчивается.
	for head < len(ordered) && len(targets) < takeCount && ordered[head].Color == targetColor {
		targets = append(targets, ordered[head])
		head++
	}

	// Всё, что не было забрано, остаётся на исходном пути.
	remainder := append([]normalized.Wagon{}, ordered[head:]...)
	return blocking, targets, remainder
}

// availableCapacity возвращает число свободных мест для вагонов на пути,
// обрезая результат снизу нулём для стабильности дальнейших проверок.
func availableCapacity(trackState TrackPlanningState) int {
	value := trackState.Track.Capacity - len(trackState.Wagons)
	if value < 0 {
		return 0
	}
	return value
}

// clonePlanningState глубоко копирует изменяемые части planning state,
// чтобы вспомогательные функции симуляции могли работать на отдельной копии,
// не изменяя состояние caller-а.
// Метаданные путей копируются по значению, а срезы вагонов клонируются и сортируются.
func clonePlanningState(state FixedClassPlanningState) FixedClassPlanningState {
	clone := state
	clone.FormationTrack = TrackPlanningState{
		Track:  state.FormationTrack.Track,
		Wagons: cloneAndSortWagons(state.FormationTrack.Wagons),
	}
	clone.BufferTrack = TrackPlanningState{
		Track:  state.BufferTrack.Track,
		Wagons: cloneAndSortWagons(state.BufferTrack.Wagons),
	}
	clone.SortingTracks = make([]TrackPlanningState, 0, len(state.SortingTracks))
	for _, trackState := range state.SortingTracks {
		clone.SortingTracks = append(clone.SortingTracks, TrackPlanningState{
			Track:  trackState.Track,
			Wagons: cloneAndSortWagons(trackState.Wagons),
		})
	}
	clone.RemainingTargetWagons = cloneAndSortWagons(state.RemainingTargetWagons)
	clone.CollectedTargetWagons = cloneAndSortWagons(state.CollectedTargetWagons)
	return clone
}

// cloneAndSortWagons возвращает клонированный срез вагонов,
// отсортированный по физической позиции и затем по WagonID
// для детерминированного tie-breaking.
//
// Правила упорядочивания:
//  1. TrackID
//  2. TrackIndex
//  3. WagonID
//
// Даже если caller обычно передаёт вагоны только с одного пути,
// использование одного общего helper-а делает planner deterministic.
func cloneAndSortWagons(wagons []normalized.Wagon) []normalized.Wagon {
	result := append([]normalized.Wagon{}, wagons...)
	sort.Slice(result, func(i, j int) bool {
		if result[i].TrackID != result[j].TrackID {
			return result[i].TrackID < result[j].TrackID
		}
		if result[i].TrackIndex != result[j].TrackIndex {
			return result[i].TrackIndex < result[j].TrackIndex
		}
		return result[i].WagonID < result[j].WagonID
	})
	return result
}

// removeCollectedTargets удаляет только что собранные вагоны
// из множества remaining-target и возвращает результат
// в каноническом отсортированном порядке.
//
// Сопоставление делается по WagonID, потому что для planner state
// важна именно идентичность вагона; TrackID/TrackIndex могут измениться
// на следующих, более сложных этапах эвристики.
func removeCollectedTargets(remaining []normalized.Wagon, collected []normalized.Wagon) []normalized.Wagon {
	if len(collected) == 0 {
		return cloneAndSortWagons(remaining)
	}
	collectedIDs := make(map[string]struct{}, len(collected))
	for _, wagon := range collected {
		collectedIDs[wagon.WagonID] = struct{}{}
	}
	result := make([]normalized.Wagon, 0, len(remaining))
	for _, wagon := range remaining {
		if _, ok := collectedIDs[wagon.WagonID]; ok {
			continue
		}
		result = append(result, wagon)
	}
	return cloneAndSortWagons(result)
}
