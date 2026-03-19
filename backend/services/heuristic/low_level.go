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
	Connections   []normalized.TrackConnection
	Couplings     map[string]struct{}
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
	StartIndex      int
	BoundaryIndex   int
	LocomotiveIndex int
	PlaceAtStart    bool
}

type lowLevelDestinationJoinPlan struct {
	Enabled       bool
	StageTrackID  string
	StageIndex    int
	JoinObject1ID string
	JoinObject2ID string
	FinalTrackID  string
	FinalIndex    int
}

type lowLevelConsistBoundary struct {
	Wagon normalized.Wagon
	Index int
}

type lowLevelOuterOrientation struct {
	LeftOuterTrackID    string
	RightOuterTrackID   string
	ExternalSideByTrack map[string]string
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

		if cutPair, ok := findBoundaryCouplingToCut(state, operation, selection); ok {
			steps = append(steps, buildCouplingScenarioStep(
				scenarioID,
				stepOrder,
				"decouple",
				cutPair[0],
				cutPair[1],
				buildLowLevelStepPayload("source_split", operation, selection.Wagons),
			))
			stepOrder++
			state.removeCoupling(cutPair[0], cutPair[1])
		}

		destinationPlacement, err := reserveDestinationPlacement(state, operation, selection)
		if err != nil {
			return nil, err
		}
		destinationJoin := planDestinationJoin(state, operation, selection, destinationPlacement)
		if err := validateImmediateCouplingPlacement(state, operation, selection, destinationPlacement, destinationJoin); err != nil {
			return nil, err
		}

		// Первый шаг skeleton-а: локомотив подъезжает к тому концу пути,
		// откуда должна быть забрана вся группа операции.
		if state.Locomotive.TrackID != operation.SourceTrackID || state.Locomotive.TrackIndex != selection.ApproachIndex {
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
		}

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
		state.addCoupling(state.Locomotive.LocoID, selection.BoundaryWagonID)

		for _, pair := range missingInternalGroupCouplings(state, selection.Wagons) {
			steps = append(steps, buildCouplingScenarioStep(
				scenarioID,
				stepOrder,
				"couple",
				pair[0],
				pair[1],
				buildLowLevelStepPayload("group_couple", operation, selection.Wagons),
			))
			stepOrder++
			state.addCoupling(pair[0], pair[1])
		}

		// Третий шаг: вся группа переводится целиком на путь назначения.
		// Мы намеренно не делим группу внутри одного draft-шага:
		// если operation говорит о переносе N вагонов, move_loco здесь описывает
		// перемещение всего этого блока сразу.
		transferLegs := []struct {
			FromTrackID string
			FromIndex   int
			ToTrackID   string
			ToIndex     int
			Phase       string
		}{
			{
				FromTrackID: operation.SourceTrackID,
				FromIndex:   selection.ApproachIndex,
				ToTrackID:   destinationJoin.finalTargetTrackID(operation.DestinationTrackID),
				ToIndex:     destinationJoin.finalTargetIndex(destinationPlacement.LocomotiveIndex),
				Phase:       "transfer",
			},
		}
		if pulloutTrackID, pulloutIndex, ok := planOuterPulloutTransfer(state, operation, selection); ok {
			transferLegs = []struct {
				FromTrackID string
				FromIndex   int
				ToTrackID   string
				ToIndex     int
				Phase       string
			}{
				{
					FromTrackID: operation.SourceTrackID,
					FromIndex:   selection.ApproachIndex,
					ToTrackID:   pulloutTrackID,
					ToIndex:     pulloutIndex,
					Phase:       "pullout",
				},
				{
					FromTrackID: pulloutTrackID,
					FromIndex:   pulloutIndex,
					ToTrackID:   destinationJoin.finalTargetTrackID(operation.DestinationTrackID),
					ToIndex:     destinationJoin.finalTargetIndex(destinationPlacement.LocomotiveIndex),
					Phase:       "transfer",
				},
			}
		}
		for _, leg := range transferLegs {
			if leg.FromTrackID == leg.ToTrackID && leg.FromIndex == leg.ToIndex {
				continue
			}
			steps = append(steps, buildMoveLocoScenarioStep(
				scenarioID,
				stepOrder,
				state.Locomotive.LocoID,
				leg.FromTrackID,
				leg.FromIndex,
				leg.ToTrackID,
				leg.ToIndex,
				buildLowLevelStepPayload(leg.Phase, operation, selection.Wagons),
			))
			stepOrder++
		}
		if destinationJoin.Enabled {
			steps = append(steps, buildCouplingScenarioStep(
				scenarioID,
				stepOrder,
				"couple",
				destinationJoin.JoinObject1ID,
				destinationJoin.JoinObject2ID,
				buildLowLevelStepPayload("destination_join", operation, selection.Wagons),
			))
			stepOrder++
			state.addCoupling(destinationJoin.JoinObject1ID, destinationJoin.JoinObject2ID)

			if destinationJoin.StageTrackID != destinationJoin.FinalTrackID || destinationJoin.StageIndex != destinationJoin.FinalIndex {
				steps = append(steps, buildMoveLocoScenarioStep(
					scenarioID,
					stepOrder,
					state.Locomotive.LocoID,
					destinationJoin.StageTrackID,
					destinationJoin.StageIndex,
					destinationJoin.FinalTrackID,
					destinationJoin.FinalIndex,
					buildLowLevelStepPayload("push_destination", operation, selection.Wagons),
				))
				stepOrder++
			}
		}

		// После генерации шагов обновляем локальное состояние,
		// чтобы следующая operation начиналась с нового положения локомотива
		// и уже переставленных вагонов.
		applyOperationTransfer(state, operation, selection, destinationPlacement, destinationJoin)
		if operation.OperationType == HeuristicOperationTransferFormationToMain {
			continue
		}

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
		state.removeCoupling(state.Locomotive.LocoID, selection.BoundaryWagonID)

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
		Connections:   append([]normalized.TrackConnection{}, scheme.TrackConnections...),
		Couplings:     buildCouplingIndex(scheme.Couplings),
	}
}

func buildCouplingIndex(items []normalized.Coupling) map[string]struct{} {
	result := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.Object1ID == "" || item.Object2ID == "" {
			continue
		}
		result[couplingKey(item.Object1ID, item.Object2ID)] = struct{}{}
	}
	return result
}

func couplingKey(a string, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "::" + b
}

func (state *lowLevelBuilderState) addCoupling(a string, b string) {
	if state.Couplings == nil {
		state.Couplings = make(map[string]struct{})
	}
	state.Couplings[couplingKey(a, b)] = struct{}{}
}

func (state *lowLevelBuilderState) removeCoupling(a string, b string) {
	if state.Couplings == nil {
		return
	}
	delete(state.Couplings, couplingKey(a, b))
}

func (state *lowLevelBuilderState) hasCoupling(a string, b string) bool {
	if state.Couplings == nil {
		return false
	}
	_, ok := state.Couplings[couplingKey(a, b)]
	return ok
}

func missingInternalGroupCouplings(state *lowLevelBuilderState, wagons []normalized.Wagon) [][2]string {
	if len(wagons) < 2 {
		return nil
	}

	ordered := cloneAndSortWagons(wagons)
	result := make([][2]string, 0, len(ordered)-1)
	for i := len(ordered) - 1; i > 0; i-- {
		a := ordered[i].WagonID
		b := ordered[i-1].WagonID
		if state.hasCoupling(a, b) {
			continue
		}
		result = append(result, [2]string{a, b})
	}
	return result
}

func findBoundaryCouplingToCut(
	state *lowLevelBuilderState,
	operation HeuristicOperation,
	selection lowLevelGroupSelection,
) ([2]string, bool) {
	sourceWagons := cloneAndSortWagons(state.WagonsByTrack[operation.SourceTrackID])
	if len(selection.Wagons) == 0 || len(sourceWagons) <= len(selection.Wagons) {
		return [2]string{}, false
	}

	selected := make(map[string]struct{}, len(selection.Wagons))
	for _, wagon := range selection.Wagons {
		selected[wagon.WagonID] = struct{}{}
	}

	switch selection.NormalizedSourceSide {
	case "start":
		lastInside := selection.Wagons[len(selection.Wagons)-1]
		for _, wagon := range sourceWagons {
			if _, ok := selected[wagon.WagonID]; ok {
				continue
			}
			if wagon.TrackIndex == lastInside.TrackIndex+1 && state.hasCoupling(lastInside.WagonID, wagon.WagonID) {
				return [2]string{lastInside.WagonID, wagon.WagonID}, true
			}
			break
		}
	case "end":
		firstInside := selection.Wagons[0]
		for i := len(sourceWagons) - 1; i >= 0; i-- {
			wagon := sourceWagons[i]
			if _, ok := selected[wagon.WagonID]; ok {
				continue
			}
			if wagon.TrackIndex == firstInside.TrackIndex-1 && state.hasCoupling(wagon.WagonID, firstInside.WagonID) {
				return [2]string{wagon.WagonID, firstInside.WagonID}, true
			}
			break
		}
	}

	return [2]string{}, false
}

func planOuterPulloutTransfer(
	state *lowLevelBuilderState,
	operation HeuristicOperation,
	selection lowLevelGroupSelection,
) (string, int, bool) {
	orientation, ok := detectLowLevelOuterOrientation(state.TracksByID, state.Connections)
	if !ok {
		return "", 0, false
	}
	if operation.SourceTrackID == operation.DestinationTrackID {
		return "", 0, false
	}
	if operation.SourceTrackID == orientation.LeftOuterTrackID || operation.SourceTrackID == orientation.RightOuterTrackID {
		return "", 0, false
	}
	if operation.DestinationTrackID == orientation.LeftOuterTrackID || operation.DestinationTrackID == orientation.RightOuterTrackID {
		return "", 0, false
	}

	attachedSide := locomotiveAttachedSideForSelection(state.TracksByID[operation.SourceTrackID], selection.NormalizedSourceSide)
	outerTrackID := orientation.RightOuterTrackID
	if attachedSide == "left" {
		outerTrackID = orientation.LeftOuterTrackID
	}

	outerTrack, ok := state.TracksByID[outerTrackID]
	if !ok {
		return "", 0, false
	}
	externalSide := orientation.ExternalSideByTrack[outerTrackID]
	if externalSide == "" {
		return "", 0, false
	}
	return outerTrackID, minimalOuterPulloutIndexForTrack(outerTrack.Capacity, externalSide, len(selection.Wagons)+1), true
}

func detectLowLevelOuterOrientation(
	tracksByID map[string]normalized.Track,
	connections []normalized.TrackConnection,
) (lowLevelOuterOrientation, bool) {
	connectedSides := map[string]map[string]int{}
	for _, connection := range connections {
		if connectedSides[connection.Track1ID] == nil {
			connectedSides[connection.Track1ID] = map[string]int{}
		}
		if connectedSides[connection.Track2ID] == nil {
			connectedSides[connection.Track2ID] = map[string]int{}
		}
		connectedSides[connection.Track1ID][connection.Track1Side]++
		connectedSides[connection.Track2ID][connection.Track2Side]++
	}

	type candidate struct {
		TrackID      string
		CenterX      float64
		ExternalSide string
	}
	candidates := make([]candidate, 0, 2)
	for _, track := range tracksByID {
		startConnected := connectedSides[track.TrackID]["start"]
		endConnected := connectedSides[track.TrackID]["end"]
		if (startConnected == 0 && endConnected == 0) || (startConnected > 0 && endConnected > 0) {
			continue
		}
		externalSide := "start"
		if startConnected > 0 {
			externalSide = "end"
		}
		candidates = append(candidates, candidate{
			TrackID:      track.TrackID,
			CenterX:      (track.StartX + track.EndX) / 2,
			ExternalSide: externalSide,
		})
	}
	if len(candidates) < 2 {
		return lowLevelOuterOrientation{}, false
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].CenterX < candidates[j].CenterX
	})
	left := candidates[0]
	right := candidates[len(candidates)-1]
	return lowLevelOuterOrientation{
		LeftOuterTrackID:  left.TrackID,
		RightOuterTrackID: right.TrackID,
		ExternalSideByTrack: map[string]string{
			left.TrackID:  left.ExternalSide,
			right.TrackID: right.ExternalSide,
		},
	}, true
}

func locomotiveAttachedSideForSelection(track normalized.Track, sourceSide string) string {
	switch sourceSide {
	case "end":
		if track.EndX >= track.StartX {
			return "right"
		}
		return "left"
	default:
		if track.StartX <= track.EndX {
			return "left"
		}
		return "right"
	}
}

func minimalOuterPulloutIndexForTrack(capacity int, externalSide string, trainLength int) int {
	if capacity <= 0 {
		return 0
	}
	if trainLength < 1 {
		trainLength = 1
	}
	maxIndex := capacity - 1
	switch externalSide {
	case "start":
		idx := maxIndex - trainLength
		if idx < 0 {
			return 0
		}
		return idx
	case "end":
		idx := trainLength
		if idx > maxIndex {
			return maxIndex
		}
		return idx
	default:
		return 0
	}
}

func planDestinationJoin(
	state *lowLevelBuilderState,
	operation HeuristicOperation,
	selection lowLevelGroupSelection,
	destinationPlacement lowLevelDestinationPlacement,
) lowLevelDestinationJoinPlan {
	existing := cloneAndSortWagons(state.WagonsByTrack[operation.DestinationTrackID])
	if len(existing) == 0 || len(selection.Wagons) == 0 || operation.OperationType == HeuristicOperationTransferFormationToMain {
		return lowLevelDestinationJoinPlan{}
	}
	boundary, ok := currentConsistBoundary(existing, destinationPlacement.PlaceAtStart)
	if !ok {
		return lowLevelDestinationJoinPlan{}
	}
	sourceTrack, sourceOK := state.TracksByID[operation.SourceTrackID]
	deliverySide := ""
	if sourceOK {
		deliverySide = locomotiveAttachedSideForSelection(sourceTrack, selection.NormalizedSourceSide)
	}

	if destinationPlacement.PlaceAtStart {
		moved := cloneAndSortWagons(selection.Wagons)
		if deliverySide != "right" && boundary.Index <= len(selection.Wagons) {
			stageTrackID, stageIndex, ok := findAdjacentLocomotivePlacement(state, operation.DestinationTrackID, "start", operation.SourceTrackID)
			if ok {
				return lowLevelDestinationJoinPlan{
					Enabled:       true,
					StageTrackID:  stageTrackID,
					StageIndex:    stageIndex,
					JoinObject1ID: moved[len(moved)-1].WagonID,
					JoinObject2ID: boundary.Wagon.WagonID,
					FinalTrackID:  operation.DestinationTrackID,
					FinalIndex:    destinationPlacement.LocomotiveIndex,
				}
			}
		}
		return lowLevelDestinationJoinPlan{
			Enabled:       true,
			StageTrackID:  operation.DestinationTrackID,
			StageIndex:    destinationPlacement.LocomotiveIndex,
			JoinObject1ID: moved[len(moved)-1].WagonID,
			JoinObject2ID: boundary.Wagon.WagonID,
			FinalTrackID:  operation.DestinationTrackID,
			FinalIndex:    destinationPlacement.LocomotiveIndex,
		}
	}

	moved := cloneAndSortWagons(selection.Wagons)
	return lowLevelDestinationJoinPlan{
		Enabled:       true,
		StageTrackID:  operation.DestinationTrackID,
		StageIndex:    destinationPlacement.LocomotiveIndex,
		JoinObject1ID: moved[0].WagonID,
		JoinObject2ID: boundary.Wagon.WagonID,
		FinalTrackID:  operation.DestinationTrackID,
		FinalIndex:    destinationPlacement.LocomotiveIndex,
	}
}

func (plan lowLevelDestinationJoinPlan) finalTargetTrackID(defaultTrackID string) string {
	if plan.Enabled {
		return plan.StageTrackID
	}
	return defaultTrackID
}

func (plan lowLevelDestinationJoinPlan) finalTargetIndex(defaultIndex int) int {
	if plan.Enabled {
		return plan.StageIndex
	}
	return defaultIndex
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
	if operation.OperationType == HeuristicOperationTransferFormationToMain {
		if resolvedSide, ok := resolveTransferFormationToMainSourceSide(state, operation, sourceTrack, wagons); ok {
			side = resolvedSide
		}
	}
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

func resolveTransferFormationToMainSourceSide(
	state *lowLevelBuilderState,
	operation HeuristicOperation,
	sourceTrack normalized.Track,
	wagons []normalized.Wagon,
) (string, bool) {
	if side, ok := resolveTransferFormationToMainEntrySide(state, operation); ok {
		return side, true
	}
	if len(wagons) == 0 {
		return "", false
	}
	firstIndex := wagons[0].TrackIndex
	lastIndex := wagons[len(wagons)-1].TrackIndex
	if state.Locomotive.TrackID == sourceTrack.TrackID {
		switch state.Locomotive.TrackIndex {
		case firstIndex - 1:
			return "start", true
		case lastIndex + 1:
			return "end", true
		}
	}
	return "", false
}

func resolveTransferFormationToMainEntrySide(
	state *lowLevelBuilderState,
	operation HeuristicOperation,
) (string, bool) {
	if operation.DestinationTrackID == "" {
		return "", false
	}
	for _, connection := range state.Connections {
		switch {
		case connection.Track1ID == operation.DestinationTrackID && connection.Track2ID != operation.DestinationTrackID:
			return connection.Track1Side, true
		case connection.Track2ID == operation.DestinationTrackID && connection.Track1ID != operation.DestinationTrackID:
			return connection.Track2Side, true
		}
	}
	return "", false
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
	reservedEntryOffset := 0
	if len(existing) == 0 {
		// На пустом пути оставляем крайний endpoint-слот свободным, чтобы группа
		// не вставала прямо в соединительный узел.
		reservedEntryOffset = 1
	}

	placeAtStart := shouldPlaceSelectionAtDestinationStart(state, operation, selection)
	startIndex := nextFreeTrackIndex(existing) + reservedEntryOffset
	boundaryIndex := startIndex
	locomotiveIndex := startIndex + len(selection.Wagons)
	if operation.OperationType == HeuristicOperationTransferFormationToMain {
		placeAtStart = shouldPlaceSelectionAtDestinationStart(state, operation, selection)
		if placeAtStart {
			startIndex = nextFreeTrackIndex(existing) + 2
			if startIndex < 2 {
				startIndex = 2
			}
			boundaryIndex = startIndex
			locomotiveIndex = startIndex - 1
		} else {
			locomotiveIndex = destinationTrack.Capacity - 2
			if locomotiveIndex < 0 {
				locomotiveIndex = 0
			}
			startIndex = locomotiveIndex - len(selection.Wagons)
			boundaryIndex = startIndex + len(selection.Wagons) - 1
		}
	} else if len(existing) > 0 {
		placement, ok := finalAdjacentJoinSlot(existing, len(selection.Wagons), placeAtStart)
		if !ok {
			return lowLevelDestinationPlacement{}, fmt.Errorf(
				"destination track %s cannot place delivered group adjacent to the current consist boundary",
				operation.DestinationTrackID,
			)
		}
		startIndex = placement.StartIndex
		boundaryIndex = placement.BoundaryIndex
		locomotiveIndex = placement.LocomotiveIndex
	} else if placeAtStart {
		startIndex = 1
		boundaryIndex = 1
		locomotiveIndex = 0
	} else {
		boundaryIndex = startIndex + len(selection.Wagons) - 1
	}

	if locomotiveIndex >= destinationTrack.Capacity || startIndex < 0 {
		return lowLevelDestinationPlacement{}, fmt.Errorf(
			"destination track %s does not have enough capacity for %d wagons and the locomotive",
			operation.DestinationTrackID,
			len(selection.Wagons),
		)
	}

	return lowLevelDestinationPlacement{
		StartIndex:      startIndex,
		BoundaryIndex:   boundaryIndex,
		LocomotiveIndex: locomotiveIndex,
		PlaceAtStart:    placeAtStart,
	}, nil
}

func currentConsistBoundary(existing []normalized.Wagon, placeAtStart bool) (lowLevelConsistBoundary, bool) {
	if len(existing) == 0 {
		return lowLevelConsistBoundary{}, false
	}
	ordered := cloneAndSortWagons(existing)
	wagon := ordered[len(ordered)-1]
	if placeAtStart {
		wagon = ordered[0]
	}
	return lowLevelConsistBoundary{
		Wagon: wagon,
		Index: wagon.TrackIndex,
	}, true
}

func finalAdjacentJoinSlot(existing []normalized.Wagon, selectionCount int, placeAtStart bool) (lowLevelDestinationPlacement, bool) {
	if selectionCount <= 0 {
		return lowLevelDestinationPlacement{}, false
	}
	boundary, ok := currentConsistBoundary(existing, placeAtStart)
	if !ok {
		return lowLevelDestinationPlacement{}, false
	}

	if placeAtStart {
		projectedExistingBoundary := boundary.Index
		requiredMin := selectionCount + 1
		if projectedExistingBoundary < requiredMin {
			projectedExistingBoundary = requiredMin
		}
		deliveredBoundary := projectedExistingBoundary - 1
		startIndex := deliveredBoundary - (selectionCount - 1)
		locomotiveIndex := startIndex - 1
		return lowLevelDestinationPlacement{
			StartIndex:      startIndex,
			BoundaryIndex:   startIndex,
			LocomotiveIndex: locomotiveIndex,
			PlaceAtStart:    true,
		}, true
	}

	startIndex := boundary.Index + 1
	return lowLevelDestinationPlacement{
		StartIndex:      startIndex,
		BoundaryIndex:   startIndex + selectionCount - 1,
		LocomotiveIndex: startIndex + selectionCount,
		PlaceAtStart:    false,
	}, true
}

func validateImmediateCouplingPlacement(
	state *lowLevelBuilderState,
	operation HeuristicOperation,
	selection lowLevelGroupSelection,
	destinationPlacement lowLevelDestinationPlacement,
	destinationJoin lowLevelDestinationJoinPlan,
) error {
	if !destinationJoin.Enabled || operation.OperationType == HeuristicOperationTransferFormationToMain {
		return nil
	}

	existing := cloneAndSortWagons(state.WagonsByTrack[operation.DestinationTrackID])
	boundary, ok := currentConsistBoundary(existing, destinationPlacement.PlaceAtStart)
	if !ok {
		return nil
	}

	expectedPlacement, ok := finalAdjacentJoinSlot(existing, len(selection.Wagons), destinationPlacement.PlaceAtStart)
	if !ok {
		return fmt.Errorf("destination join placement could not be resolved for track %s", operation.DestinationTrackID)
	}

	expectedJoinObject2 := boundary.Wagon.WagonID
	expectedJoinObject1 := cloneAndSortWagons(selection.Wagons)[0].WagonID
	if destinationPlacement.PlaceAtStart {
		moved := cloneAndSortWagons(selection.Wagons)
		expectedJoinObject1 = moved[len(moved)-1].WagonID
	}

	actualMovedBoundary := destinationPlacement.StartIndex
	actualExistingBoundary := boundary.Index
	if destinationPlacement.PlaceAtStart {
		projectedExistingBoundary := boundary.Index
		requiredMin := len(selection.Wagons) + 1
		if projectedExistingBoundary < requiredMin {
			projectedExistingBoundary = requiredMin
		}
		actualExistingBoundary = projectedExistingBoundary
		actualMovedBoundary = destinationPlacement.StartIndex + len(selection.Wagons) - 1
	} else {
		actualMovedBoundary = destinationPlacement.StartIndex
	}

	if absInt(actualExistingBoundary-actualMovedBoundary) != 1 {
		return fmt.Errorf(
			"destination join is emitted before adjacency is guaranteed: destination=%s existing_boundary=%d delivered_boundary=%d",
			operation.DestinationTrackID,
			actualExistingBoundary,
			actualMovedBoundary,
		)
	}
	if destinationJoin.JoinObject1ID != expectedJoinObject1 || destinationJoin.JoinObject2ID != expectedJoinObject2 {
		return fmt.Errorf(
			"destination join uses stale boundary objects on track %s: expected %s-%s, got %s-%s",
			operation.DestinationTrackID,
			expectedJoinObject1,
			expectedJoinObject2,
			destinationJoin.JoinObject1ID,
			destinationJoin.JoinObject2ID,
		)
	}
	if expectedPlacement.StartIndex != destinationPlacement.StartIndex || expectedPlacement.LocomotiveIndex != destinationPlacement.LocomotiveIndex {
		return fmt.Errorf(
			"destination placement is not aligned with the current consist boundary on track %s",
			operation.DestinationTrackID,
		)
	}

	return nil
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func shouldPlaceSelectionAtDestinationStart(
	state *lowLevelBuilderState,
	operation HeuristicOperation,
	selection lowLevelGroupSelection,
) bool {
	if operation.OperationType == HeuristicOperationTransferFormationToMain {
		if entrySide, ok := resolveTransferFormationToMainEntrySide(state, operation); ok {
			return entrySide == "start"
		}
		return true
	}

	sourceTrack, sourceOK := state.TracksByID[operation.SourceTrackID]
	destinationTrack, destinationOK := state.TracksByID[operation.DestinationTrackID]
	if !sourceOK || !destinationOK {
		return selection.NormalizedSourceSide == "start"
	}

	deliverySide := locomotiveAttachedSideForSelection(sourceTrack, selection.NormalizedSourceSide)
	startIsLeft := destinationTrack.StartX <= destinationTrack.EndX
	if deliverySide == "left" {
		return startIsLeft
	}
	return !startIsLeft
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
	destinationJoin lowLevelDestinationJoinPlan,
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
	state.WagonsByTrack[operation.SourceTrackID] = cloneAndSortWagons(sourceRemainder)

	movedStartIndex := destinationPlacement.StartIndex
	// Builder state stores the final placement after the whole delivery operation.
	// If we staged via a throat and then pushed onto the destination track, the
	// delivered wagons end up immediately after the locomotive on the target path,
	// not one slot deeper at the temporary throat-stage geometry.
	if destinationPlacement.PlaceAtStart && destinationJoin.Enabled && destinationJoin.StageTrackID != destinationJoin.FinalTrackID {
		movedStartIndex = destinationPlacement.LocomotiveIndex + 1
	}

	movedGroup := make([]normalized.Wagon, 0, len(selection.Wagons))
	for index, wagon := range cloneAndSortWagons(selection.Wagons) {
		next := wagon
		next.TrackID = operation.DestinationTrackID
		next.TrackIndex = movedStartIndex + index
		movedGroup = append(movedGroup, next)
	}

	existingDestination := cloneAndSortWagons(state.WagonsByTrack[operation.DestinationTrackID])
	destinationWagons := make([]normalized.Wagon, 0, len(existingDestination)+len(movedGroup))
	if destinationPlacement.PlaceAtStart {
		destinationWagons = append(destinationWagons, movedGroup...)
		if len(existingDestination) > 0 {
			existingMin := existingDestination[0].TrackIndex
			desiredExistingStart := movedStartIndex + len(movedGroup)
			shift := desiredExistingStart - existingMin
			if shift < 0 {
				shift = 0
			}
			for _, wagon := range existingDestination {
				next := wagon
				next.TrackIndex = wagon.TrackIndex + shift
				destinationWagons = append(destinationWagons, next)
			}
		}
	} else {
		destinationWagons = append(destinationWagons, existingDestination...)
		destinationWagons = append(destinationWagons, movedGroup...)
	}
	state.WagonsByTrack[operation.DestinationTrackID] = cloneAndSortWagons(destinationWagons)

	state.Locomotive.TrackID = operation.DestinationTrackID
	state.Locomotive.TrackIndex = destinationPlacement.LocomotiveIndex
	if destinationJoin.Enabled {
		state.addCoupling(destinationJoin.JoinObject1ID, destinationJoin.JoinObject2ID)
	}
}

func findAdjacentLocomotivePlacement(
	state *lowLevelBuilderState,
	destinationTrackID string,
	destinationSide string,
	preferredSourceTrackID string,
) (string, int, bool) {
	type candidate struct {
		TrackID string
		Index   int
		Score   int
	}
	candidates := make([]candidate, 0, 2)
	for _, connection := range state.Connections {
		var otherTrackID string
		var otherSide string
		switch {
		case connection.Track1ID == destinationTrackID && connection.Track1Side == destinationSide:
			otherTrackID = connection.Track2ID
			otherSide = connection.Track2Side
		case connection.Track2ID == destinationTrackID && connection.Track2Side == destinationSide:
			otherTrackID = connection.Track1ID
			otherSide = connection.Track1Side
		default:
			continue
		}
		otherTrack, ok := state.TracksByID[otherTrackID]
		if !ok {
			continue
		}
		index := 0
		switch otherSide {
		case "start":
			index = 1
			if index >= otherTrack.Capacity {
				index = otherTrack.Capacity - 1
			}
		case "end":
			index = otherTrack.Capacity - 2
			if index < 0 {
				index = 0
			}
		default:
			continue
		}
		score := 1
		if otherTrackID == preferredSourceTrackID {
			score = 0
		}
		candidates = append(candidates, candidate{
			TrackID: otherTrackID,
			Index:   index,
			Score:   score,
		})
	}
	if len(candidates) == 0 {
		return "", 0, false
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score < candidates[j].Score
		}
		if candidates[i].TrackID != candidates[j].TrackID {
			return candidates[i].TrackID < candidates[j].TrackID
		}
		return candidates[i].Index < candidates[j].Index
	})
	return candidates[0].TrackID, candidates[0].Index, true
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
