package heuristic

// Этот файл реализует ШАГ 5 первой backend-эвристики.
//
// Его задача — превратить high-level heuristic actions в более конкретные
// промежуточные доменные операции, которые позже станут входом для scenario
// builder. На этом этапе операции уже явно указывают источник, назначение,
// сторону подхода и количество вагонов, но всё ещё не являются низкоуровневыми
// командами маневрового исполнения.
//
// Важно:
//   - здесь НЕ создаются scenario_steps
//   - здесь НЕ строятся move_loco / couple / decouple
//   - здесь НЕ выполняется path finding
//   - здесь НЕ делается execution integration
//
// На этом уровне мы описываем только доменные операции вида:
//   - убрать блокирующие вагоны в буфер
//   - перевести целевые вагоны на путь формирования
//   - перевести готовый состав с пути формирования на главный путь

// HeuristicOperationType задаёт тип промежуточной доменной операции.
//
// Эти типы уже конкретнее, чем HeuristicActionType, потому что отражают
// направленную операцию "откуда -> куда", но всё ещё не содержат деталей
// исполнения на уровне отдельных сценарных шагов.
type HeuristicOperationType string

const (
	// HeuristicOperationBufferBlockers означает перенос blocking-вагонов
	// с исходного пути на буферный путь.
	HeuristicOperationBufferBlockers HeuristicOperationType = "buffer_blockers"

	// HeuristicOperationTransferTargetsToFormation означает перенос target-вагонов
	// с исходного sorting track на formation track.
	HeuristicOperationTransferTargetsToFormation HeuristicOperationType = "transfer_targets_to_formation"

	// HeuristicOperationTransferFormationToMain означает перевод готового
	// сформированного состава с formation track на main track.
	HeuristicOperationTransferFormationToMain HeuristicOperationType = "transfer_formation_to_main"
)

// HeuristicOperation описывает промежуточную доменную операцию, которая позже
// может быть развёрнута в low-level сценарные шаги.
//
// Значение полей:
//   - OperationType: тип доменной операции
//   - SourceTrackID: путь-источник операции
//   - DestinationTrackID: путь-назначение операции
//   - SourceSide: сторона исходного пути, если операция зависит от стороны подхода
//   - WagonCount: количество вагонов, участвующих в операции
//   - TargetColor: целевой цвет, если операция относится к target-группе
//   - FormationTrackID: путь формирования, важный для контекста операции
//   - BufferTrackID: буферный путь, важный для контекста операции
//   - MainTrackID: главный путь, важный для финального перевода
//
// Ожидаемые инварианты:
//   - для buffer_blockers WagonCount соответствует BlockingCount из действия
//   - для transfer_targets_to_formation WagonCount соответствует TakeCount
//   - для transfer_formation_to_main SourceTrackID == FormationTrackID
//   - DestinationTrackID заполнен для всех поддержанных типов операций
//   - операции остаются декларативными и ничего не исполняют сами по себе
type HeuristicOperation struct {
	OperationType      HeuristicOperationType
	SourceTrackID      string
	DestinationTrackID string
	SourceSide         string
	WagonCount         int
	TargetColor        string
	FormationTrackID   string
	BufferTrackID      string
	MainTrackID        string
}

// BuildHeuristicOperations превращает high-level action plan в ordered список
// промежуточных доменных операций.
//
// Входные данные:
//   - problem: валидированная fixed-class задача; из неё берётся TargetColor
//     и канонические идентификаторы formation / buffer / main tracks
//   - actions: ранее построенный high-level plan действий эвристики
//
// Правила преобразования:
//   - move_blockers_to_buffer -> одна операция buffer_blockers
//   - extract_target_group_to_formation -> одна операция transfer_targets_to_formation
//   - final_transfer_formation_to_main -> одна операция transfer_formation_to_main
//
// Ограничения:
//   - функция не пытается объединять, оптимизировать или симулировать операции
//   - функция не создаёт низкоуровневые команды движения
//   - функция сохраняет исходный порядок действий, потому что он важен для
//     следующего этапа сценарного построения
func BuildHeuristicOperations(problem FixedClassProblem, actions []HeuristicAction) []HeuristicOperation {
	operations := make([]HeuristicOperation, 0, len(actions))

	for _, action := range actions {
		switch action.ActionType {
		case HeuristicActionMoveBlockersToBuffer:
			operations = append(operations, HeuristicOperation{
				OperationType:      HeuristicOperationBufferBlockers,
				SourceTrackID:      action.SourceTrackID,
				DestinationTrackID: action.BufferTrackID,
				SourceSide:         action.SourceSide,
				WagonCount:         action.BlockingCount,
				TargetColor:        problem.TargetColor,
				FormationTrackID:   action.FormationTrackID,
				BufferTrackID:      action.BufferTrackID,
				MainTrackID:        action.MainTrackID,
			})
		case HeuristicActionExtractTargetGroupToFormation:
			operations = append(operations, HeuristicOperation{
				OperationType:      HeuristicOperationTransferTargetsToFormation,
				SourceTrackID:      action.SourceTrackID,
				DestinationTrackID: action.FormationTrackID,
				SourceSide:         action.SourceSide,
				WagonCount:         action.TakeCount,
				TargetColor:        problem.TargetColor,
				FormationTrackID:   action.FormationTrackID,
				BufferTrackID:      action.BufferTrackID,
				MainTrackID:        action.MainTrackID,
			})
		case HeuristicActionFinalTransferFormationToMain:
			operations = append(operations, HeuristicOperation{
				OperationType:      HeuristicOperationTransferFormationToMain,
				SourceTrackID:      action.FormationTrackID,
				DestinationTrackID: action.MainTrackID,
				SourceSide:         action.SourceSide,
				WagonCount:         action.TakeCount,
				TargetColor:        problem.TargetColor,
				FormationTrackID:   action.FormationTrackID,
				BufferTrackID:      action.BufferTrackID,
				MainTrackID:        action.MainTrackID,
			})
		}
	}

	return operations
}
