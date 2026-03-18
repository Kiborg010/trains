package heuristic

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"trains/backend/normalized"
)

// Этот файл реализует ШАГ 12: первый low-level skeleton builder поверх
// уже готовых heuristic operations.
//
// Задача файла:
//   - не менять верхнеуровневую эвристику
//   - не пытаться сразу строить идеальный исполнимый маневровый план
//   - но превратить каждую доменную heuristic operation в обычные scenario_steps
//     формата ручного сценария:
//   - move_loco
//   - couple
//   - decouple
//
// Важная граница ответственности:
//   - здесь НЕ выполняется execution integration
//   - здесь НЕ строится точная реалистичная маршрутизация по всем промежуточным путям
//   - здесь НЕ моделируются все edge cases сцепок и разъездов
//   - здесь НЕ оптимизируется последовательность шагов
//
// Вместо этого builder создаёт упрощённый, но уже структурно правильный skeleton:
//   - подъехать локомотивом к нужному концу пути
//   - сцепиться с крайней вагонной группой
//   - перевести всю группу целиком на путь назначения
//   - расцепиться
//
// Такой результат уже хранится в стандартной модели scenarios/scenario_steps
// и больше не использует абстрактный move_group.

// lowLevelBuilderState хранит минимальное изменяемое состояние, необходимое
// для последовательного разворачивания heuristic operations в обычные шаги сценария.
//
// Зачем нужна эта структура:
//   - операции зависят от текущего положения локомотива
//   - каждая предыдущая операция меняет расположение вагонов по путям
//   - следующий шаг должен строиться уже от обновлённого состояния, а не от исходной схемы
//
// Инварианты:
//   - Locomotive всегда содержит текущее предполагаемое положение локомотива
//   - WagonsByTrack содержит текущее упорядоченное состояние вагонов по путям
//   - срезы в WagonsByTrack поддерживаются отсортированными по TrackIndex
type lowLevelBuilderState struct {
	Locomotive    normalized.Locomotive
	WagonsByTrack map[string][]normalized.Wagon
	TracksByID    map[string]normalized.Track
}

// lowLevelGroupSelection описывает ту вагонную группу, с которой builder работает
// внутри одной heuristic operation.
//
// Поля нужны для двух задач одновременно:
//   - построить правильные object ids в couple/decouple
//   - переставить именно эту группу в локальном builder state после transfer-шага
//
// Инварианты:
//   - Wagons всегда содержит целостную группу заданной операции
//   - BoundaryWagonID — это вагон, с которым локомотив сцепляется на выбранной стороне
//   - SourceBoundaryIndex — индекс этого крайнего вагона на исходном пути
type lowLevelGroupSelection struct {
	Wagons               []normalized.Wagon
	BoundaryWagonID      string
	SourceBoundaryIndex  int
	ApproachIndex        int
	NormalizedSourceSide string
}

// lowLevelDestinationPlacement описывает, как именно builder собирается
// положить переносимую группу на путь назначения.
//
// Почему это выделено в отдельную структуру:
//   - одного только "первого свободного индекса" недостаточно, чтобы сохранять
//     смысл состава при prepend/append операциях
//   - builder должен отдельно знать:
//   - с какого индекса начинается новая группа
//   - рядом с каким индексом после переноса окажется локомотив
//   - кладётся ли группа в голову или в хвост текущего состава
//
// Инварианты:
//   - StartIndex и BoundaryIndex относятся уже к новому состоянию пути назначения
//   - PlaceAtStart == true означает prepend новой группы перед существующими вагонами пути
type lowLevelDestinationPlacement struct {
	StartIndex    int
	BoundaryIndex int
	PlaceAtStart  bool
}

// BuildLowLevelScenarioStepsFromHeuristicOperations строит первый низкоуровневый
// skeleton обычных scenario_steps из набора heuristic operations.
//
// Вход:
//   - scenarioID: id стандартного сценария, в который будут записаны шаги
//   - scheme: нормализованная схема; используется как source of truth для track ids и capacity
//   - operations: ordered heuristic operations, уже построенные верхними этапами эвристики
//   - currentLocomotive: текущее положение локомотива, от которого начинается skeleton
//   - currentWagons: текущее положение вагонов по путям
//
// Что делает функция:
//   - инициализирует упрощённое внутреннее состояние путей
//   - для каждой operation выбирает целостную вагонную группу
//   - разворачивает operation в последовательность:
//     1. move_loco к исходной группе
//     2. couple
//     3. move_loco с группой на путь назначения
//     4. decouple
//   - обновляет локальное состояние, чтобы следующая operation строилась уже от нового положения
//
// Ограничения текущей версии:
//   - destination placement упрощённо делается в первый свободный индекс на пути назначения
//   - сложная геометрия и реальный путь движения здесь не рассчитываются
//   - существующие couplings в схеме не анализируются глубоко
//   - builder предполагает, что heuristic operation уже описывает перенос всей группы целиком
//
// Возвращает:
//   - ordered список обычных normalized.ScenarioStep
//   - ошибку, если исходные данные не позволяют построить даже skeleton
func BuildLowLevelScenarioStepsFromHeuristicOperations(
	scenarioID string,
	scheme normalized.Scheme,
	operations []HeuristicOperation,
	currentLocomotive normalized.Locomotive,
	currentWagons []normalized.Wagon,
) ([]normalized.ScenarioStep, error) {
	if scenarioID == "" {
		return nil, fmt.Errorf("scenario id is required")
	}
	if len(operations) == 0 {
		return []normalized.ScenarioStep{}, nil
	}
	if currentLocomotive.LocoID == "" {
		return nil, fmt.Errorf("current locomotive is required for low-level skeleton builder")
	}

	state := newLowLevelBuilderState(scheme, currentLocomotive, currentWagons)
	steps := make([]normalized.ScenarioStep, 0, len(operations)*4)
	stepOrder := 0

	for _, operation := range operations {
		selection, err := selectOperationGroup(state, operation)
		if err != nil {
			return nil, err
		}

		destinationPlacement, err := reserveDestinationPlacement(state, operation, selection)
		if err != nil {
			return nil, err
		}

		// Первый шаг skeleton-а: локомотив подъезжает к тому концу пути,
		// откуда должна быть забрана вся группа операции.
		steps = append(steps, buildMoveLocoScenarioStep(
			scenarioID,
			stepOrder,
			state.Locomotive.LocoID,
			state.Locomotive.TrackID,
			state.Locomotive.TrackIndex,
			operation.SourceTrackID,
			selection.ApproachIndex,
			buildLowLevelStepPayload("approach", operation, selection.Wagons),
		))
		stepOrder++

		// Второй шаг: локомотив сцепляется именно с крайним вагоном группы,
		// доступным со стороны operation.SourceSide.
		steps = append(steps, buildCouplingScenarioStep(
			scenarioID,
			stepOrder,
			"couple",
			state.Locomotive.LocoID,
			selection.BoundaryWagonID,
			buildLowLevelStepPayload("couple", operation, selection.Wagons),
		))
		stepOrder++

		// Третий шаг: вся группа переводится целиком на путь назначения.
		// Мы намеренно не делим группу внутри одного draft-шага:
		// если operation говорит о переносе N вагонов, move_loco здесь описывает
		// перемещение всего этого блока сразу.
		steps = append(steps, buildMoveLocoScenarioStep(
			scenarioID,
			stepOrder,
			state.Locomotive.LocoID,
			operation.SourceTrackID,
			selection.SourceBoundaryIndex,
			operation.DestinationTrackID,
			destinationPlacement.BoundaryIndex,
			buildLowLevelStepPayload("transfer", operation, selection.Wagons),
		))
		stepOrder++

		// Четвёртый шаг: локомотив отцепляется от того же крайнего вагона,
		// с которым он был связан в рамках этой операции.
		steps = append(steps, buildCouplingScenarioStep(
			scenarioID,
			stepOrder,
			"decouple",
			state.Locomotive.LocoID,
			selection.BoundaryWagonID,
			buildLowLevelStepPayload("decouple", operation, selection.Wagons),
		))
		stepOrder++

		// После генерации шагов обновляем локальное состояние,
		// чтобы следующая operation начиналась с нового положения локомотива
		// и уже переставленных вагонов.
		applyOperationTransfer(state, operation, selection, destinationPlacement)
	}

	return steps, nil
}

// newLowLevelBuilderState строит стартовое внутреннее состояние builder-а
// из нормализованной схемы и текущих позиций локомотива/вагонов.
//
// Builder не использует напрямую всю схему при каждой операции,
// поэтому здесь заранее создаются удобные индексы:
//   - TracksByID
//   - WagonsByTrack
func newLowLevelBuilderState(
	scheme normalized.Scheme,
	currentLocomotive normalized.Locomotive,
	currentWagons []normalized.Wagon,
) *lowLevelBuilderState {
	tracksByID := make(map[string]normalized.Track, len(scheme.Tracks))
	for _, track := range scheme.Tracks {
		tracksByID[track.TrackID] = track
	}

	wagonsByTrack := make(map[string][]normalized.Wagon)
	for _, wagon := range currentWagons {
		wagonsByTrack[wagon.TrackID] = append(wagonsByTrack[wagon.TrackID], wagon)
	}
	for trackID := range wagonsByTrack {
		wagonsByTrack[trackID] = cloneAndSortWagons(wagonsByTrack[trackID])
	}

	return &lowLevelBuilderState{
		Locomotive:    currentLocomotive,
		WagonsByTrack: wagonsByTrack,
		TracksByID:    tracksByID,
	}
}

// selectOperationGroup выбирает фактическую группу вагонов,
// которую low-level skeleton должен считать переносимой целиком.
//
// Правила текущей версии:
//   - для start берётся начало пути
//   - для end берётся конец пути
//   - buffer_blockers берёт WagonCount крайних вагонов независимо от цвета
//   - transfer_targets_to_formation дополнительно проверяет, что выбранная группа
//     действительно состоит из targetColor
//   - transfer_formation_to_main берёт WagonCount вагонов с пути формирования;
//     если SourceSide не указан, используется "start" как детерминированное значение по умолчанию
func selectOperationGroup(state *lowLevelBuilderState, operation HeuristicOperation) (lowLevelGroupSelection, error) {
	sourceTrack, ok := state.TracksByID[operation.SourceTrackID]
	if !ok {
		return lowLevelGroupSelection{}, fmt.Errorf("source track %s was not found in scheme", operation.SourceTrackID)
	}

	wagons := cloneAndSortWagons(state.WagonsByTrack[sourceTrack.TrackID])
	if len(wagons) == 0 {
		return lowLevelGroupSelection{}, fmt.Errorf("source track %s has no wagons for operation %s", operation.SourceTrackID, operation.OperationType)
	}
	if operation.WagonCount <= 0 {
		return lowLevelGroupSelection{}, fmt.Errorf("operation %s must move a positive wagon_count", operation.OperationType)
	}
	if operation.WagonCount > len(wagons) {
		return lowLevelGroupSelection{}, fmt.Errorf(
			"operation %s requests %d wagons from track %s, but only %d are available",
			operation.OperationType,
			operation.WagonCount,
			operation.SourceTrackID,
			len(wagons),
		)
	}

	side := strings.TrimSpace(operation.SourceSide)
	if side == "" {
		resolvedSide, err := resolveImplicitSourceSide(sourceTrack, wagons)
		if err != nil {
			return lowLevelGroupSelection{}, err
		}
		side = resolvedSide
	}

	if operation.OperationType == HeuristicOperationTransferFormationToMain && operation.WagonCount != len(wagons) {
		return lowLevelGroupSelection{}, fmt.Errorf(
			"formation-to-main must move the whole formation: requested %d wagons from %s, but formation currently has %d wagons",
			operation.WagonCount,
			operation.SourceTrackID,
			len(wagons),
		)
	}

	var group []normalized.Wagon
	switch side {
	case "start":
		group = append([]normalized.Wagon{}, wagons[:operation.WagonCount]...)
	case "end":
		group = append([]normalized.Wagon{}, wagons[len(wagons)-operation.WagonCount:]...)
	default:
		return lowLevelGroupSelection{}, fmt.Errorf("unsupported source side %q", side)
	}

	if operation.OperationType == HeuristicOperationTransferTargetsToFormation {
		for _, wagon := range group {
			if wagon.Color != operation.TargetColor {
				return lowLevelGroupSelection{}, fmt.Errorf(
					"target transfer from track %s selected non-target wagon %s with color %s",
					operation.SourceTrackID,
					wagon.WagonID,
					wagon.Color,
				)
			}
		}
	}

	boundaryWagon := group[0]
	if side == "end" {
		boundaryWagon = group[len(group)-1]
	}

	approachIndex, err := computeApproachIndex(sourceTrack, wagons, side, boundaryWagon.TrackIndex)
	if err != nil {
		return lowLevelGroupSelection{}, err
	}

	return lowLevelGroupSelection{
		Wagons:               group,
		BoundaryWagonID:      boundaryWagon.WagonID,
		SourceBoundaryIndex:  boundaryWagon.TrackIndex,
		ApproachIndex:        approachIndex,
		NormalizedSourceSide: side,
	}, nil
}

// resolveImplicitSourceSide выбирает сторону подхода для операций,
// у которых source_side явно не задан.
//
// На этом этапе builder использует максимально простое правило:
//   - если перед первым вагоном есть свободный индекс, используем start
//   - иначе, если после последнего вагона есть свободный индекс, используем end
//   - иначе возвращаем ошибку
func resolveImplicitSourceSide(sourceTrack normalized.Track, sourceWagons []normalized.Wagon) (string, error) {
	if len(sourceWagons) == 0 {
		return "", fmt.Errorf("cannot resolve source side on empty track %s", sourceTrack.TrackID)
	}

	firstIndex := sourceWagons[0].TrackIndex
	lastIndex := sourceWagons[len(sourceWagons)-1].TrackIndex
	if firstIndex-1 >= 0 && !isTrackIndexOccupied(sourceWagons, firstIndex-1) {
		return "start", nil
	}
	if lastIndex+1 < sourceTrack.Capacity && !isTrackIndexOccupied(sourceWagons, lastIndex+1) {
		return "end", nil
	}
	return "", fmt.Errorf("cannot resolve a free adjacent source side on track %s", sourceTrack.TrackID)
}

// computeApproachIndex рассчитывает индекс, на который должен приехать локомотив
// перед сцепкой с выбранной группой.
//
// Правило этой правки простое:
//   - для start локомотив встаёт перед первым вагоном группы
//   - для end локомотив встаёт после последнего вагона группы
//
// Важное ограничение:
// локомотив не должен вставать в индекс, уже занятый вагоном, и не должен
// выходить за допустимые границы пути.
func computeApproachIndex(
	sourceTrack normalized.Track,
	sourceWagons []normalized.Wagon,
	side string,
	boundaryIndex int,
) (int, error) {
	approachIndex := boundaryIndex - 1
	if side == "end" {
		approachIndex = boundaryIndex + 1
	}

	if approachIndex < 0 || approachIndex >= sourceTrack.Capacity {
		return 0, fmt.Errorf(
			"cannot place locomotive adjacent to group on track %s: approach index %d is outside capacity %d",
			sourceTrack.TrackID,
			approachIndex,
			sourceTrack.Capacity,
		)
	}

	if isTrackIndexOccupied(sourceWagons, approachIndex) {
		return 0, fmt.Errorf(
			"cannot place locomotive adjacent to group on track %s: approach index %d is already occupied",
			sourceTrack.TrackID,
			approachIndex,
		)
	}

	return approachIndex, nil
}

func isTrackIndexOccupied(wagons []normalized.Wagon, targetIndex int) bool {
	for _, wagon := range wagons {
		if wagon.TrackIndex == targetIndex {
			return true
		}
	}
	return false
}

// reserveDestinationPlacement рассчитывает, куда на пути назначения будет
// поставлена переносимая группа.
//
// Текущая упрощённая модель deliberately простая:
//   - группа ставится в первый свободный индекс после уже занятых вагонов
//   - индексы группы идут подряд
//   - capacity пути назначения проверяется до генерации шагов
//
// Возвращает:
//   - destinationStartIndex: индекс первого вагона группы на пути назначения
//   - destinationBoundaryIndex: индекс того вагона группы, рядом с которым после transfer
//     окажется локомотив и который потом участвует в decouple
func reserveDestinationPlacement(
	state *lowLevelBuilderState,
	operation HeuristicOperation,
	selection lowLevelGroupSelection,
) (lowLevelDestinationPlacement, error) {
	destinationTrack, ok := state.TracksByID[operation.DestinationTrackID]
	if !ok {
		return lowLevelDestinationPlacement{}, fmt.Errorf("destination track %s was not found in scheme", operation.DestinationTrackID)
	}

	existing := cloneAndSortWagons(state.WagonsByTrack[destinationTrack.TrackID])
	existingCount := len(existing)
	if existingCount+len(selection.Wagons) > destinationTrack.Capacity {
		return lowLevelDestinationPlacement{}, fmt.Errorf(
			"destination track %s does not have enough capacity for %d wagons",
			operation.DestinationTrackID,
			len(selection.Wagons),
		)
	}

	placeAtStart := selection.NormalizedSourceSide == "start"
	startIndex := existingCount
	boundaryIndex := existingCount
	if placeAtStart {
		startIndex = 0
		boundaryIndex = 0
	} else {
		boundaryIndex = existingCount + len(selection.Wagons) - 1
	}

	return lowLevelDestinationPlacement{
		StartIndex:    startIndex,
		BoundaryIndex: boundaryIndex,
		PlaceAtStart:  placeAtStart,
	}, nil
}

// applyOperationTransfer обновляет локальное builder state после того,
// как skeleton для операции уже сгенерирован.
//
// Мы физически переносим выбранную группу из одного TrackID в другой,
// переназначаем TrackIndex подряд и обновляем положение локомотива.
// Это нужно, чтобы следующая операция брала данные уже из нового состояния.
func applyOperationTransfer(
	state *lowLevelBuilderState,
	operation HeuristicOperation,
	selection lowLevelGroupSelection,
	destinationPlacement lowLevelDestinationPlacement,
) {
	selectedByID := make(map[string]struct{}, len(selection.Wagons))
	for _, wagon := range selection.Wagons {
		selectedByID[wagon.WagonID] = struct{}{}
	}

	sourceRemainder := make([]normalized.Wagon, 0, len(state.WagonsByTrack[operation.SourceTrackID]))
	for _, wagon := range state.WagonsByTrack[operation.SourceTrackID] {
		if _, moved := selectedByID[wagon.WagonID]; moved {
			continue
		}
		sourceRemainder = append(sourceRemainder, wagon)
	}
	state.WagonsByTrack[operation.SourceTrackID] = compactTrackWagons(operation.SourceTrackID, sourceRemainder)

	movedGroup := make([]normalized.Wagon, 0, len(selection.Wagons))
	for index, wagon := range cloneAndSortWagons(selection.Wagons) {
		next := wagon
		next.TrackID = operation.DestinationTrackID
		next.TrackIndex = destinationPlacement.StartIndex + index
		movedGroup = append(movedGroup, next)
	}

	existingDestination := cloneAndSortWagons(state.WagonsByTrack[operation.DestinationTrackID])
	destinationWagons := make([]normalized.Wagon, 0, len(existingDestination)+len(movedGroup))
	if destinationPlacement.PlaceAtStart {
		destinationWagons = append(destinationWagons, movedGroup...)
		destinationWagons = append(destinationWagons, existingDestination...)
	} else {
		destinationWagons = append(destinationWagons, existingDestination...)
		destinationWagons = append(destinationWagons, movedGroup...)
	}
	state.WagonsByTrack[operation.DestinationTrackID] = compactTrackWagons(operation.DestinationTrackID, destinationWagons)

	state.Locomotive.TrackID = operation.DestinationTrackID
	state.Locomotive.TrackIndex = destinationPlacement.BoundaryIndex
}

// nextFreeTrackIndex возвращает первый свободный TrackIndex на пути.
//
// Функция не пытается заполнять дырки в индексах, а просто продолжает
// текущий максимум. Для skeleton builder это безопаснее и проще:
// мы не переуплотняем уже существующее размещение вагонов.
func nextFreeTrackIndex(wagons []normalized.Wagon) int {
	if len(wagons) == 0 {
		return 0
	}
	maxIndex := wagons[0].TrackIndex
	for _, wagon := range wagons[1:] {
		if wagon.TrackIndex > maxIndex {
			maxIndex = wagon.TrackIndex
		}
	}
	return maxIndex + 1
}

// compactTrackWagons пересобирает состояние одного пути в детерминированный
// компактный вид: вагоны сохраняют относительный порядок, но их TrackIndex
// перенумеровываются подряд от нуля.
//
// Это нужно builder-у по двум причинам:
//   - после синтетического переноса групп не оставлять разрывы в индексах
//   - сделать следующий расчёт destination placement предсказуемым
func compactTrackWagons(trackID string, wagons []normalized.Wagon) []normalized.Wagon {
	ordered := cloneAndSortWagons(wagons)
	result := make([]normalized.Wagon, 0, len(ordered))
	for index, wagon := range ordered {
		next := wagon
		next.TrackID = trackID
		next.TrackIndex = index
		result = append(result, next)
	}
	return result
}

// buildMoveLocoScenarioStep создаёт стандартный scenario_step типа move_loco.
//
// Несмотря на то, что runtime сейчас использует в основном to_track_id/to_index,
// мы дополнительно записываем real from_track_id/from_index, чтобы skeleton
// был самодостаточным и читаемым как обычный сценарий.
func buildMoveLocoScenarioStep(
	scenarioID string,
	stepOrder int,
	locomotiveID string,
	fromTrackID string,
	fromIndex int,
	toTrackID string,
	toIndex int,
	payload json.RawMessage,
) normalized.ScenarioStep {
	stepID := fmt.Sprintf("%s-step-%03d", scenarioID, stepOrder)
	return normalized.ScenarioStep{
		StepID:      stepID,
		ScenarioID:  scenarioID,
		StepOrder:   stepOrder,
		StepType:    "move_loco",
		FromTrackID: stringPtr(fromTrackID),
		FromIndex:   intPtr(fromIndex),
		ToTrackID:   stringPtr(toTrackID),
		ToIndex:     intPtr(toIndex),
		Object1ID:   stringPtr(locomotiveID),
		PayloadJSON: payload,
	}
}

// buildCouplingScenarioStep создаёт стандартный scenario_step типа couple или decouple.
//
// Для обоих случаев builder использует одну и ту же пару объектов:
//   - object1_id = локомотив
//   - object2_id = крайний вагон группы
func buildCouplingScenarioStep(
	scenarioID string,
	stepOrder int,
	stepType string,
	object1ID string,
	object2ID string,
	payload json.RawMessage,
) normalized.ScenarioStep {
	stepID := fmt.Sprintf("%s-step-%03d", scenarioID, stepOrder)
	return normalized.ScenarioStep{
		StepID:      stepID,
		ScenarioID:  scenarioID,
		StepOrder:   stepOrder,
		StepType:    stepType,
		Object1ID:   stringPtr(object1ID),
		Object2ID:   stringPtr(object2ID),
		PayloadJSON: payload,
	}
}

// buildLowLevelStepPayload добавляет в каждый шаг минимальный debug/context payload,
// чтобы потом было проще понять, из какой heuristic operation он появился.
//
// Это не меняет поведение исполнения, но делает сохранённый сценарий
// гораздо понятнее при отладке и просмотре через API/UI.
func buildLowLevelStepPayload(
	phase string,
	operation HeuristicOperation,
	group []normalized.Wagon,
) json.RawMessage {
	wagonIDs := make([]string, 0, len(group))
	for _, wagon := range group {
		wagonIDs = append(wagonIDs, wagon.WagonID)
	}
	sort.Strings(wagonIDs)

	payload, err := json.Marshal(map[string]any{
		"heuristic_operation_type": operation.OperationType,
		"phase":                    phase,
		"source_side":              operation.SourceSide,
		"wagon_count":              operation.WagonCount,
		"target_color":             operation.TargetColor,
		"formation_track_id":       operation.FormationTrackID,
		"buffer_track_id":          operation.BufferTrackID,
		"main_track_id":            operation.MainTrackID,
		"wagon_ids":                wagonIDs,
	})
	if err != nil {
		return nil
	}
	return payload
}

func stringPtr(value string) *string {
	return &value
}

func intPtr(value int) *int {
	return &value
}
