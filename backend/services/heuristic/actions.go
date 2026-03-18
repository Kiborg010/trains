package heuristic

// Этот файл реализует ШАГ 4 первой backend-эвристики.
//
// Его задача — превратить уже выбранный порядок извлечения целевых групп
// в высокоуровневый план действий, который можно будет позже разворачивать
// в низкоуровневые манёвры.
//
// Важно:
//   - здесь НЕ создаются scenario_steps
//   - здесь НЕ генерируются move_loco / couple / decouple
//   - здесь НЕ выполняется поиск маршрута
//   - здесь НЕ моделируется исполнение манёвров
//
// На этом этапе мы строим только декларативный план вида:
//   1. убрать блокировки в буфер
//   2. извлечь целевую группу на путь формирования
//   3. после набора K вагонов перевести сформированный состав на главный путь

// HeuristicActionType задаёт тип высокоуровневого действия эвристики.
//
// Эти типы являются переходным слоем между абстрактным планом извлечения
// и будущим low-level planner'ом, который уже будет строить scenario_steps.
type HeuristicActionType string

const (
	// HeuristicActionMoveBlockersToBuffer означает, что перед извлечением target-группы
	// необходимо временно убрать blocking вагоны на buffer track.
	HeuristicActionMoveBlockersToBuffer HeuristicActionType = "move_blockers_to_buffer"

	// HeuristicActionExtractTargetGroupToFormation означает, что доступная target-группа
	// должна быть переведена с sorting track на formation track.
	HeuristicActionExtractTargetGroupToFormation HeuristicActionType = "extract_target_group_to_formation"

	// HeuristicActionFinalTransferFormationToMain означает, что после набора K target-вагонов
	// состав на formation track считается готовым к переводу на main track.
	HeuristicActionFinalTransferFormationToMain HeuristicActionType = "final_transfer_formation_to_main"
)

// HeuristicAction описывает одно высокоуровневое действие эвристики.
//
// Структура хранит не команды исполнения, а именно семантику шага,
// достаточную для следующего этапа, где эти действия будут разворачиваться
// в конкретные манёвры и scenario_steps.
//
// Значение полей:
//   - ActionType: тип действия
//   - SourceTrackID: исходный путь, с которого действие логически выполняется
//   - SourceSide: конец исходного пути, если действие зависит от направления подхода
//   - BufferTrackID: буферный путь, используемый для временного размещения блокировок
//   - FormationTrackID: путь формирования целевого состава
//   - MainTrackID: главный путь, на который в конце должен быть выведен состав
//   - BlockingCount: сколько blocking-вагонов связано с этим действием
//   - TakeCount: сколько target-вагонов действие предполагает взять в состав
//   - TargetGroupSize: размер доступной целевой группы, обнаруженной на исходном пути
//
// Ожидаемые инварианты:
//   - для move_blockers_to_buffer BlockingCount > 0, а TakeCount обычно равен 0
//   - для extract_target_group_to_formation TakeCount > 0
//   - для final_transfer_formation_to_main SourceTrackID обычно совпадает с FormationTrackID
//   - BufferTrackID / FormationTrackID / MainTrackID должны быть заполнены в корректном плане
//   - структура сама по себе ничего не исполняет и не гарантирует физическую выполнимость манёвра
type HeuristicAction struct {
	ActionType       HeuristicActionType
	SourceTrackID    string
	SourceSide       string
	BufferTrackID    string
	FormationTrackID string
	MainTrackID      string
	BlockingCount    int
	TakeCount        int
	TargetGroupSize  int
}

// BuildHighLevelHeuristicPlan строит упорядоченный высокоуровневый план действий
// по уже выбранному ordered extraction plan.
//
// Входные данные:
//   - problem: валидированное описание fixed-class задачи, из которого берутся
//     идентификаторы formation / buffer / main tracks
//   - initialState: planning state на момент старта высокоуровневого планирования;
//     нужен, чтобы понимать, сколько target-вагонов уже собрано на formation track
//   - orderedPlan: ранее выбранный порядок extraction decisions
//
// Правила построения плана:
//   - если у extraction decision есть blocking вагоны, сначала добавляется
//     move_blockers_to_buffer
//   - затем всегда добавляется extract_target_group_to_formation
//   - как только число собранных target-вагонов достигает K, добавляется
//     final_transfer_formation_to_main и план завершается
//
// Ограничения:
//   - функция не строит никакие low-level movement команды
//   - функция не проверяет маршрут или физическую достижимость
//   - функция только преобразует уже выбранные решения в более явный action plan
func BuildHighLevelHeuristicPlan(
	problem FixedClassProblem,
	initialState FixedClassPlanningState,
	orderedPlan []TargetExtractionCandidate,
) []HeuristicAction {
	actions := make([]HeuristicAction, 0, len(orderedPlan)*2+1)
	currentCollectedCount := initialState.CurrentCollectedCount

	for _, decision := range orderedPlan {
		// Если перед target-группой есть blocking вагоны, то на high-level это
		// выражается отдельным действием перемещения блокировок в буфер.
		if decision.BlockingCount > 0 {
			actions = append(actions, HeuristicAction{
				ActionType:       HeuristicActionMoveBlockersToBuffer,
				SourceTrackID:    decision.SourceSortingTrackID,
				SourceSide:       decision.SourceSide,
				BufferTrackID:    problem.BufferTrack.TrackID,
				FormationTrackID: problem.FormationTrack.TrackID,
				MainTrackID:      problem.MainTrack.TrackID,
				BlockingCount:    decision.BlockingCount,
				TakeCount:        0,
				TargetGroupSize:  decision.TargetGroupSize,
			})
		}

		// После удаления блокировок или сразу, если их нет, добавляется действие
		// по извлечению доступной target-группы на путь формирования.
		actions = append(actions, HeuristicAction{
			ActionType:       HeuristicActionExtractTargetGroupToFormation,
			SourceTrackID:    decision.SourceSortingTrackID,
			SourceSide:       decision.SourceSide,
			BufferTrackID:    problem.BufferTrack.TrackID,
			FormationTrackID: problem.FormationTrack.TrackID,
			MainTrackID:      problem.MainTrack.TrackID,
			BlockingCount:    decision.BlockingCount,
			TakeCount:        decision.TakeCount,
			TargetGroupSize:  decision.TargetGroupSize,
		})

		currentCollectedCount += decision.TakeCount

		// Как только достигается K, high-level planner считает, что состав на
		// formation track собран, и добавляет финальное действие перевода на main.
		if currentCollectedCount >= initialState.RequiredTargetCount {
			actions = append(actions, HeuristicAction{
				ActionType:       HeuristicActionFinalTransferFormationToMain,
				SourceTrackID:    problem.FormationTrack.TrackID,
				SourceSide:       "",
				BufferTrackID:    problem.BufferTrack.TrackID,
				FormationTrackID: problem.FormationTrack.TrackID,
				MainTrackID:      problem.MainTrack.TrackID,
				BlockingCount:    0,
				TakeCount:        currentCollectedCount,
				TargetGroupSize:  currentCollectedCount,
			})
			break
		}
	}

	return actions
}
