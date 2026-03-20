package heuristic

import (
	"fmt"
	"sort"
	"strings"

	"trains/backend/normalized"
)

const (
	minSortingTrackCount = 2
	maxSortingTrackCount = 10
	minLeadTrackCount    = 2
	maxLeadTrackCount    = 10
)

// Этот файл реализует первые два этапа эвристики для фиксированного минимального класса схем.
//
// Область ответственности файла:
//   - ШАГ 1: построение нормализованного и проверенного "описания задачи" для
//     ограниченного класса станции, который поддерживает первая версия эвристики.
//   - ШАГ 2: явная проверка реализуемости задачи до начала какого-либо планирования.
//
// Фиксированный минимальный класс схемы здесь намеренно узкий:
//   - ровно 1 главный путь
//   - ровно 1 объездной путь
//   - ровно 2 сортировочных пути
//   - ровно 2 вытяжных пути
//   - хранение на главном и объездном путях запрещено
//   - хранение на сортировочных и вытяжных путях разрешено
//   - цвета вагонов ограничены целевым цветом и ровно одним нецелевым цветом
//
// Основные задачи этого файла:
//   - классифицировать нормализованные пути по функциональным ролям
//   - проверить, что схема соответствует форме, поддерживаемой эвристикой
//   - разделить вагоны на целевые и нецелевые
//   - выбрать вытяжной путь формирования и буферный вытяжной путь
//   - сообщить, является ли задача реализуемой для заданного размера состава K
//
// Что здесь уже реализовано:
//   - строгая проверка фиксированного класса схемы
//   - проверка цветов вагонов и индексация вагонов по путям
//   - явный результат проверки реализуемости с объяснением причин
//   - прозрачные правила выбора formation/buffer путей
//
// Что здесь специально пока не реализовано:
//   - порядок извлечения вагонов
//   - планирование маневров
//   - генерация scenario_steps
//   - интеграция с низкоуровневым движением/выполнением

// FixedClassProblem — это нормализованная входная модель задачи для первой эвристики.
//
// Эта структура описывает состояние станции только после того, как схема уже
// прошла базовые проверки фиксированного класса, выполняемые в BuildFixedClassProblem.
// Следующие этапы планирования работают с этими полями, а не с исходной схемой.
//
// Значение полей:
//   - SchemeID: идентификатор схемы, сохраняется для трассировки
//   - TargetColor: цвет, который эвристика должна собрать в итоговый состав
//   - MainTrack: единственный главный путь в поддерживаемом классе схем
//   - BypassTrack: единственный объездной путь в поддерживаемом классе схем
//   - SortingTracks: ровно два сортировочных пути, отсортированные по TrackID
//   - LeadTracks: ровно два вытяжных пути, отсортированные по TrackID
//   - FormationTrack: вытяжной путь, выбранный для накопления целевого состава
//   - BufferTrack: второй вытяжной путь, используемый как временный буфер
//   - TargetWagons: все вагоны, у которых Color совпадает с TargetColor
//   - NonTargetWagons: все вагоны, у которых Color отличается от TargetColor
//   - WagonsByTrack: вагоны, сгруппированные по TrackID и отсортированные по TrackIndex
//
// Ожидаемые инварианты после успешного построения:
//   - MainTrack и BypassTrack уникальны
//   - len(SortingTracks) == 2
//   - len(LeadTracks) == 2
//   - FormationTrack и BufferTrack — это два вытяжных пути в некотором порядке
//   - все срезы вагонов в WagonsByTrack отсортированы по TrackIndex
//   - TargetWagons не пуст
//   - в схеме используется не более двух цветов вагонов
type FixedClassProblem struct {
	SchemeID         int
	TargetColor      string
	Tracks           []normalized.Track
	TracksByID       map[string]normalized.Track
	TrackConnections []normalized.TrackConnection
	MainTrack        normalized.Track
	BypassTrack      normalized.Track
	SortingTracks    []normalized.Track
	LeadTracks       []normalized.Track
	FormationTrack   normalized.Track
	BufferTrack      normalized.Track
	TargetWagons     []normalized.Wagon
	NonTargetWagons  []normalized.Wagon
	WagonsByTrack    map[string][]normalized.Wagon
}

// FixedClassFeasibility — явный результат проверки реализуемости задачи
// для эвристики фиксированного минимального класса.
//
// Идея в том, чтобы можно было ответить на вопрос
// "можно ли вообще применять эту эвристику?"
// ещё до запуска какого-либо планирования. Результат намеренно подробный,
// чтобы его можно было использовать в UI, тестах или будущей orchestration-логике.
//
// Значение полей:
//   - Feasible: true только если все проверки пройдены
//   - Reasons: накопленные объяснения, почему задача нереализуема
//   - ChosenFormationTrackID: выбранный вытяжной путь для накопления целевого состава
//   - ChosenBufferTrackID: второй вытяжной путь, зарезервированный под буфер
//   - TargetCount: число вагонов требуемого цвета
//   - RequiredTargetCount: требуемое значение K для целевого состава
//   - AvailableBufferCapacity: свободная вместимость выбранного буферного пути
//
// Ожидаемые инварианты:
//   - если Feasible == true, Reasons пуст
//   - если ChosenFormationTrackID не пуст, он относится к вытяжному пути
//   - если ChosenBufferTrackID не пуст, он относится ко второму вытяжному пути
//   - AvailableBufferCapacity всегда неотрицателен
type FixedClassFeasibility struct {
	Feasible                bool
	Reasons                 []string
	ChosenFormationTrackID  string
	ChosenBufferTrackID     string
	TargetCount             int
	RequiredTargetCount     int
	AvailableBufferCapacity int
}

// BuildFixedClassProblem проверяет, что нормализованная схема соответствует
// классу схем, поддерживаемых первой эвристикой, а затем строит удобное
// внутреннее представление задачи для следующих этапов.
//
// Ожидаемый вход:
//   - scheme: нормализованная схема, содержащая пути и вагоны
//   - targetColor: цвет, который нужно собрать
//   - formationTrackID: может быть пустым; тогда первый отсортированный lead-путь
//     выбирается через chooseLeadTracks; более строгий выбор делается позже
//
// Основные предположения:
//   - поддерживается только ограниченный фиксированный минимальный класс схем
//   - цвета вагонов ограничены целевым цветом и максимум одним другим цветом
//   - позиции вагонов уже нормализованы через TrackID/TrackIndex
//
// Ошибки могут возникать в случаях:
//   - не задан целевой цвет
//   - неправильное число путей по типам
//   - неверные флаги хранения у путей
//   - неверный formationTrackID
//   - пустой цвет у вагона
//   - слишком много цветов вагонов или отсутствие target-вагонов
//
// Эта функция находится в самом начале pipeline эвристики:
// она подготавливает стабильный и проверенный входной объект
// до этапов проверки реализуемости и планирования.
func BuildFixedClassProblem(scheme normalized.Scheme, targetColor string, formationTrackID string) (FixedClassProblem, error) {
	// Целевой цвет — обязательный параметр верхнего уровня.
	// Обрезка пробелов здесь нужна, чтобы избежать ложных успехов
	// из-за строк, отличающихся только пробелами.
	targetColor = strings.TrimSpace(targetColor)
	if targetColor == "" {
		return FixedClassProblem{}, fmt.Errorf("нужно указать целевой цвет")
	}

	// Разделяем пути по смысловым ролям. Эвристика работает не с произвольными
	// TrackID, а именно с ролями путей, поэтому классификация — первый шаг.
	mainTracks := make([]normalized.Track, 0)
	bypassTracks := make([]normalized.Track, 0)
	sortingTracks := make([]normalized.Track, 0)
	leadTracks := make([]normalized.Track, 0)

	for _, track := range scheme.Tracks {
		// На всякий случай обрезаем пробелы вокруг типа пути,
		// чтобы случайные артефакты данных не ломали классификацию.
		switch strings.TrimSpace(track.Type) {
		case "main":
			mainTracks = append(mainTracks, track)
		case "bypass":
			bypassTracks = append(bypassTracks, track)
		case "sorting":
			sortingTracks = append(sortingTracks, track)
		case "lead":
			leadTracks = append(leadTracks, track)
		}
	}

	// Сортируем все группы путей, чтобы дальнейшие решения были детерминированными.
	// Это особенно полезно в тестах и в ситуациях, когда путь formation
	// явно не указан.
	sortTracks(mainTracks)
	sortTracks(bypassTracks)
	sortTracks(sortingTracks)
	sortTracks(leadTracks)

	// Первая эвристика поддерживает ровно одну форму схемы.
	// Любое отклонение лучше отклонять сразу, чем получать неочевидное
	// поведение позже на этапе планирования.
	if len(mainTracks) != 1 {
		return FixedClassProblem{}, fmt.Errorf("ожидался ровно 1 главный путь, получено %d", len(mainTracks))
	}
	if len(bypassTracks) != 1 {
		return FixedClassProblem{}, fmt.Errorf("ожидался ровно 1 обходной путь, получено %d", len(bypassTracks))
	}
	if err := validateTrackCountInRange("sorting", len(sortingTracks), minSortingTrackCount, maxSortingTrackCount); err != nil {
		return FixedClassProblem{}, err
	}
	if err := validateTrackCountInRange("lead", len(leadTracks), minLeadTrackCount, maxLeadTrackCount); err != nil {
		return FixedClassProblem{}, err
	}

	mainTrack := mainTracks[0]
	bypassTrack := bypassTracks[0]
	// Семантика хранения — часть контракта эвристики.
	// Если эти флаги неверные, дальнейшие предположения о планировании
	// становятся невалидными, поэтому завершаемся сразу.
	if mainTrack.StorageAllowed {
		return FixedClassProblem{}, fmt.Errorf("главный путь %s не должен допускать хранение", mainTrack.TrackID)
	}
	if bypassTrack.StorageAllowed {
		return FixedClassProblem{}, fmt.Errorf("обходной путь %s не должен допускать хранение", bypassTrack.TrackID)
	}
	for _, track := range sortingTracks {
		if !track.StorageAllowed {
			return FixedClassProblem{}, fmt.Errorf("сортировочный путь %s должен допускать хранение", track.TrackID)
		}
	}
	for _, track := range leadTracks {
		if !track.StorageAllowed {
			return FixedClassProblem{}, fmt.Errorf("вытяжной путь %s должен допускать хранение", track.TrackID)
		}
	}

	// Назначение formation/buffer — единственное решение на уровне lead-путей
	// на этом этапе. Вспомогательная функция либо валидирует явный выбор,
	// либо применяет детерминированный выбор по умолчанию.
	formationTrack, bufferTrack, err := chooseLeadTracks(leadTracks, formationTrackID)
	if err != nil {
		return FixedClassProblem{}, err
	}

	// За один проход строим:
	//   - разбиение вагонов на target/non-target
	//   - индекс вагонов по путям
	// Последующие этапы работают в основном с WagonsByTrack
	// и уже готовыми срезами target/non-target.
	targetWagons := make([]normalized.Wagon, 0)
	nonTargetWagons := make([]normalized.Wagon, 0)
	wagonsByTrack := make(map[string][]normalized.Wagon)
	tracksByID := make(map[string]normalized.Track, len(scheme.Tracks))
	for _, track := range scheme.Tracks {
		tracksByID[track.TrackID] = track
	}
	for _, wagon := range scheme.Wagons {
		// Пустой цвет запрещён явно, потому что эвристика опирается
		// на бинарное разделение target/non-target и не умеет
		// обрабатывать "бесцветные" вагоны.
		color := strings.TrimSpace(wagon.Color)
		if color == "" {
			return FixedClassProblem{}, fmt.Errorf("у вагона %s не указан цвет", wagon.WagonID)
		}
		wagonsByTrack[wagon.TrackID] = append(wagonsByTrack[wagon.TrackID], wagon)
		if color == targetColor {
			targetWagons = append(targetWagons, wagon)
		} else {
			nonTargetWagons = append(nonTargetWagons, wagon)
		}
	}

	// Первая эвристика намеренно узкая:
	// допускается максимум два цвета и при этом хотя бы один target-вагон.
	// Иначе планировать просто нечего.
	if len(targetWagons) == 0 {
		return FixedClassProblem{}, fmt.Errorf("вагоны целевого цвета %q не найдены", targetColor)
	}

	// Сортируем вагоны на каждом пути по TrackIndex, чтобы в следующих этапах
	// порядок элементов в срезе соответствовал физическому порядку вагонов.
	for trackID := range wagonsByTrack {
		sort.Slice(wagonsByTrack[trackID], func(i, j int) bool {
			return wagonsByTrack[trackID][i].TrackIndex < wagonsByTrack[trackID][j].TrackIndex
		})
	}

	return FixedClassProblem{
		SchemeID:         scheme.SchemeID,
		TargetColor:      targetColor,
		Tracks:           append([]normalized.Track{}, scheme.Tracks...),
		TracksByID:       tracksByID,
		TrackConnections: append([]normalized.TrackConnection{}, scheme.TrackConnections...),
		MainTrack:        mainTrack,
		BypassTrack:      bypassTrack,
		SortingTracks:    append([]normalized.Track{}, sortingTracks...),
		LeadTracks:       append([]normalized.Track{}, leadTracks...),
		FormationTrack:   formationTrack,
		BufferTrack:      bufferTrack,
		TargetWagons:     append([]normalized.Wagon{}, targetWagons...),
		NonTargetWagons:  append([]normalized.Wagon{}, nonTargetWagons...),
		WagonsByTrack:    wagonsByTrack,
	}, nil
}

// CheckFixedClassFeasibility выполняет "непорождающую" проверку:
// можно ли вообще применять эту эвристику к данной схеме.
//
// В отличие от BuildFixedClassProblem, эта функция не падает на первой ошибке,
// а накапливает причины нереализуемости. Благодаря этому её удобно использовать
// как объясняющий слой валидации перед запуском planner-а.
//
// Вход:
//   - scheme: нормализованное состояние станции
//   - targetColor: цвет, который нужно собрать
//   - requiredTargetCount: желаемая длина состава K
//   - formationTrackID: необязательный явно заданный lead-путь для формирования
//
// Возвращает:
//   - объект FixedClassFeasibility, содержащий и итоговый verdict,
//     и конкретные причины/выбранные пути
//
// Положение в pipeline:
//   - это gate между чтением схемы и этапом планирования
func CheckFixedClassFeasibility(scheme normalized.Scheme, targetColor string, requiredTargetCount int, formationTrackID string) FixedClassFeasibility {
	result := FixedClassFeasibility{
		Feasible:            false,
		Reasons:             []string{},
		RequiredTargetCount: requiredTargetCount,
	}

	// K должен быть осмысленной целью планирования.
	// Ноль или отрицательное значение превращают задачу в вырожденный случай
	// и скрывают ошибки выше по стеку.
	if requiredTargetCount <= 0 {
		result.Reasons = append(result.Reasons, "required target count K must be positive")
		return result
	}

	// Целевой цвет обязателен по той же причине, что и в BuildFixedClassProblem:
	// все рассуждения о target/non-target опираются на него.
	targetColor = strings.TrimSpace(targetColor)
	if targetColor == "" {
		result.Reasons = append(result.Reasons, "target color is required")
		return result
	}

	// Повторно классифицируем пути прямо из raw normalized tracks.
	// Проверка реализуемости специально сделана независимой от уже построенного
	// FixedClassProblem, поэтому не использует его как вход.
	mainTracks := make([]normalized.Track, 0)
	bypassTracks := make([]normalized.Track, 0)
	sortingTracks := make([]normalized.Track, 0)
	leadTracks := make([]normalized.Track, 0)
	for _, track := range scheme.Tracks {
		switch strings.TrimSpace(track.Type) {
		case "main":
			mainTracks = append(mainTracks, track)
		case "bypass":
			bypassTracks = append(bypassTracks, track)
		case "sorting":
			sortingTracks = append(sortingTracks, track)
		case "lead":
			leadTracks = append(leadTracks, track)
		}
	}
	sortTracks(mainTracks)
	sortTracks(bypassTracks)
	sortTracks(sortingTracks)
	sortTracks(leadTracks)

	// Сначала проверяется сама форма схемы.
	// Если схема не соответствует фиксированному классу,
	// дальше проверять более частные условия нет смысла.
	if len(mainTracks) != 1 {
		result.Reasons = append(result.Reasons, fmt.Sprintf("expected exactly 1 main track, got %d", len(mainTracks)))
	}
	if len(bypassTracks) != 1 {
		result.Reasons = append(result.Reasons, fmt.Sprintf("expected exactly 1 bypass track, got %d", len(bypassTracks)))
	}
	if err := validateTrackCountInRange("sorting", len(sortingTracks), minSortingTrackCount, maxSortingTrackCount); err != nil {
		result.Reasons = append(result.Reasons, err.Error())
	}
	if err := validateTrackCountInRange("lead", len(leadTracks), minLeadTrackCount, maxLeadTrackCount); err != nil {
		result.Reasons = append(result.Reasons, err.Error())
	}
	if len(result.Reasons) > 0 {
		return result
	}

	// Правила хранения являются базовой частью постановки:
	// main и bypass используются для движения/вывода, а sorting и lead —
	// для хранения и промежуточных перестановок.
	if mainTracks[0].StorageAllowed {
		result.Reasons = append(result.Reasons, fmt.Sprintf("main track %s must not allow storage", mainTracks[0].TrackID))
	}
	if bypassTracks[0].StorageAllowed {
		result.Reasons = append(result.Reasons, fmt.Sprintf("bypass track %s must not allow storage", bypassTracks[0].TrackID))
	}
	for _, track := range sortingTracks {
		if !track.StorageAllowed {
			result.Reasons = append(result.Reasons, fmt.Sprintf("sorting track %s must allow storage", track.TrackID))
		}
	}
	for _, track := range leadTracks {
		if !track.StorageAllowed {
			result.Reasons = append(result.Reasons, fmt.Sprintf("lead track %s must allow storage", track.TrackID))
		}
	}

	// За один проход:
	//   - считаем число target-вагонов
	//   - собираем множество цветов
	//   - оцениваем текущую занятость путей
	// Занятость потом используется при выборе formation/buffer.
	targetCount := 0
	occupiedByTrack := map[string]int{}
	for _, wagon := range scheme.Wagons {
		color := strings.TrimSpace(wagon.Color)
		if color == "" {
			result.Reasons = append(result.Reasons, fmt.Sprintf("wagon %s has empty color", wagon.WagonID))
			continue
		}
		occupiedByTrack[wagon.TrackID]++
		if color == targetColor {
			targetCount++
		}
	}
	result.TargetCount = targetCount

	// Первая эвристика определена только для мира из двух цветов:
	// целевой цвет + один "все остальные".
	// Меньше двух цветов на непустой станции тоже считается недопустимым,
	// потому что текущий контракт ожидает явное разделение на два класса.
	if targetCount < requiredTargetCount {
		result.Reasons = append(result.Reasons, fmt.Sprintf("not enough target wagons: have %d, need %d", targetCount, requiredTargetCount))
	}

	// Выбираем или валидируем пару formation/buffer.
	// Вспомогательная функция возвращает не только пути,
	// но и причины отказа, чтобы caller мог показать их прозрачно.
	formationTrack, bufferTrack, chooseReasons := selectFormationAndBufferTracks(leadTracks, occupiedByTrack, requiredTargetCount, formationTrackID)
	result.Reasons = append(result.Reasons, chooseReasons...)
	if formationTrack.TrackID != "" {
		result.ChosenFormationTrackID = formationTrack.TrackID
	}
	if bufferTrack.TrackID != "" {
		result.ChosenBufferTrackID = bufferTrack.TrackID
		// Вместимость буфера измеряется как число свободных ячеек,
		// а не полная capacity.
		// Если входные данные уже переполнены, отрицательные значения
		// обрезаются до нуля.
		bufferCapacity := bufferTrack.Capacity - occupiedByTrack[bufferTrack.TrackID]
		if bufferCapacity < 0 {
			bufferCapacity = 0
		}
		result.AvailableBufferCapacity = bufferCapacity
		if bufferCapacity <= 0 {
			result.Reasons = append(result.Reasons, fmt.Sprintf("buffer track %s has no available capacity", bufferTrack.TrackID))
		}
	}

	// Если накопились какие-либо причины отказа, задача считается нереализуемой.
	// Иначе явно помечаем её как feasible.
	if len(result.Reasons) > 0 {
		return result
	}
	result.Feasible = true
	return result
}

// chooseLeadTracks валидирует явно выбранный formation-путь
// или применяет детерминированный порядок по умолчанию,
// если caller не указал его явно.
//
// Эта функция специально остаётся минимальной:
//   - она рассуждает только об идентичности двух lead-путей
//   - она не проверяет вместимость и занятость
//   - такие policy-решения принадлежат selectFormationAndBufferTracks
func chooseLeadTracks(leadTracks []normalized.Track, formationTrackID string) (normalized.Track, normalized.Track, error) {
	if len(leadTracks) < minLeadTrackCount {
		return normalized.Track{}, normalized.Track{}, fmt.Errorf("нужно минимум %d вытяжных пути", minLeadTrackCount)
	}
	// Пустой выбор означает:
	// "использовать детерминированный порядок по умолчанию".
	formationTrackID = strings.TrimSpace(formationTrackID)
	if formationTrackID == "" {
		return leadTracks[0], leadTracks[1], nil
	}
	for _, track := range leadTracks {
		if track.TrackID != formationTrackID {
			continue
		}
		bufferTrack, ok := chooseBufferTrack(leadTracks, formationTrackID, nil)
		if !ok {
			return normalized.Track{}, normalized.Track{}, fmt.Errorf("буферный путь не найден")
		}
		return track, bufferTrack, nil
	}
	return normalized.Track{}, normalized.Track{}, fmt.Errorf("путь формирования %s не относится к вытяжным путям", formationTrackID)
}

// sortTracks сортирует пути по TrackID, чтобы все последующие решения были
// стабильными и детерминированными. Это низкоуровневая вспомогательная функция,
// используемая до применения более высокоуровневых правил выбора и tie-break.
func sortTracks(tracks []normalized.Track) {
	sort.Slice(tracks, func(i, j int) bool {
		return tracks[i].TrackID < tracks[j].TrackID
	})
}

// selectFormationAndBufferTracks выбирает lead-путь для формирования состава
// и второй lead-путь, который будет использоваться как буфер.
//
// Правила выбора:
//   - если formationTrackID задан, он должен указывать на один из двух lead-путей
//   - выбранный formation-путь должен иметь capacity >= requiredTargetCount
//   - если путь явно не задан, кандидаты на formation — только те lead-пути,
//     у которых capacity >= K
//   - среди кандидатов приоритеты такие:
//     1. меньшая текущая занятость
//     2. большая общая вместимость
//     3. лексикографически меньший TrackID
//
// Возвращаемые причины объясняют, почему не удалось выбрать безопасную пару путей.
func selectFormationAndBufferTracks(
	leadTracks []normalized.Track,
	occupiedByTrack map[string]int,
	requiredTargetCount int,
	formationTrackID string,
) (normalized.Track, normalized.Track, []string) {
	reasons := []string{}
	if len(leadTracks) < minLeadTrackCount {
		return normalized.Track{}, normalized.Track{}, []string{fmt.Sprintf("at least %d lead tracks are required", minLeadTrackCount)}
	}
	if len(leadTracks) > maxLeadTrackCount {
		return normalized.Track{}, normalized.Track{}, []string{fmt.Sprintf("at most %d lead tracks are allowed", maxLeadTrackCount)}
	}

	// Если caller явно указал formation-путь, мы валидируем этот выбор,
	// а второй lead автоматически становится буфером.
	formationTrackID = strings.TrimSpace(formationTrackID)
	if formationTrackID != "" {
		var formationTrack normalized.Track
		found := false
		for _, track := range leadTracks {
			if track.TrackID == formationTrackID {
				formationTrack = track
				found = true
				break
			}
		}
		if !found {
			return normalized.Track{}, normalized.Track{}, []string{fmt.Sprintf("formation track %s is not one of the lead tracks", formationTrackID)}
		}
		bufferTrack, ok := chooseBufferTrack(leadTracks, formationTrack.TrackID, occupiedByTrack)
		if !ok {
			return normalized.Track{}, normalized.Track{}, []string{"буферный путь не найден"}
		}
		if formationTrack.Capacity < requiredTargetCount {
			reasons = append(reasons, fmt.Sprintf(
				"formation track %s capacity %d is less than required target count %d",
				formationTrack.TrackID,
				formationTrack.Capacity,
				requiredTargetCount,
			))
		}
		return formationTrack, bufferTrack, reasons
	}

	// Если явного выбора нет, то кандидатом на formation может быть только тот lead,
	// который физически способен вместить весь целевой состав длины K.
	candidates := make([]normalized.Track, 0, len(leadTracks))
	for _, track := range leadTracks {
		if track.Capacity >= requiredTargetCount {
			candidates = append(candidates, track)
		}
	}
	if len(candidates) == 0 {
		return normalized.Track{}, normalized.Track{}, []string{
			fmt.Sprintf("no lead track has capacity >= %d", requiredTargetCount),
		}
	}

	// Предпочитаем путь с меньшей текущей занятостью,
	// потому что он требует меньше предварительных маневров.
	// Если занятость одинакова — выбираем путь с большей вместимостью,
	// затем — с меньшим TrackID для детерминированности.
	sort.Slice(candidates, func(i, j int) bool {
		leftOccupied := occupiedByTrack[candidates[i].TrackID]
		rightOccupied := occupiedByTrack[candidates[j].TrackID]
		if leftOccupied != rightOccupied {
			return leftOccupied < rightOccupied
		}
		if candidates[i].Capacity != candidates[j].Capacity {
			return candidates[i].Capacity > candidates[j].Capacity
		}
		return candidates[i].TrackID < candidates[j].TrackID
	})

	formationTrack := candidates[0]
	bufferTrack, ok := chooseBufferTrack(leadTracks, formationTrack.TrackID, occupiedByTrack)
	if !ok {
		return normalized.Track{}, normalized.Track{}, []string{"буферный путь не найден"}
	}

	return formationTrack, bufferTrack, nil
}

func validateTrackCountInRange(trackKind string, count int, min int, max int) error {
	switch {
	case count < min:
		return fmt.Errorf("ожидалось минимум %d путей типа %s, получено %d", min, trackKind, count)
	case count > max:
		return fmt.Errorf("ожидалось максимум %d путей типа %s, получено %d", max, trackKind, count)
	default:
		return nil
	}
}

func chooseBufferTrack(
	leadTracks []normalized.Track,
	formationTrackID string,
	occupiedByTrack map[string]int,
) (normalized.Track, bool) {
	candidates := make([]normalized.Track, 0, len(leadTracks))
	for _, track := range leadTracks {
		if track.TrackID == formationTrackID {
			continue
		}
		candidates = append(candidates, track)
	}
	if len(candidates) == 0 {
		return normalized.Track{}, false
	}
	sort.Slice(candidates, func(i, j int) bool {
		leftAvailable := candidates[i].Capacity
		rightAvailable := candidates[j].Capacity
		if occupiedByTrack != nil {
			leftAvailable -= occupiedByTrack[candidates[i].TrackID]
			rightAvailable -= occupiedByTrack[candidates[j].TrackID]
		}
		if leftAvailable != rightAvailable {
			return leftAvailable > rightAvailable
		}
		if candidates[i].Capacity != candidates[j].Capacity {
			return candidates[i].Capacity > candidates[j].Capacity
		}
		return candidates[i].TrackID < candidates[j].TrackID
	})
	return candidates[0], true
}
