package heuristic

// Этот файл реализует ШАГ 6 первой backend-эвристики.
//
// Его задача — превратить набор промежуточных доменных операций в черновой
// сценарий эвристики. Такой сценарий уже имеет упорядоченные шаги, но всё ещё
// не является исполнимым набором low-level movement-команд.
//
// Важно:
//   - здесь НЕ создаются normalized scenario_steps
//   - здесь НЕ генерируются move_loco / couple / decouple
//   - здесь НЕ выполняется path finding
//   - здесь НЕ делается execution integration
//
// На этом этапе мы получаем draft-представление сценария, которое удобно для
// следующих этапов: оно уже упорядочено и содержит доменные параметры каждого
// шага, но ещё не привязано к конкретной low-level механике исполнения.

// DraftScenarioStepType задаёт тип шага в черновом эвристическом сценарии.
//
// Типы шагов здесь намеренно совпадают по смыслу с доменными операциями:
// draft scenario пока лишь фиксирует порядок и параметры действий, не переводя
// их в исполнимые маневровые команды.
type DraftScenarioStepType string

const (
	// DraftScenarioStepBufferBlockers означает шаг по временной уборке
	// blocking-вагонов на буферный путь.
	DraftScenarioStepBufferBlockers DraftScenarioStepType = "buffer_blockers"

	// DraftScenarioStepTransferTargetsToFormation означает шаг по переводу
	// target-вагонов с sorting track на formation track.
	DraftScenarioStepTransferTargetsToFormation DraftScenarioStepType = "transfer_targets_to_formation"

	// DraftScenarioStepTransferFormationToMain означает финальный шаг по переводу
	// собранного состава с formation track на main track.
	DraftScenarioStepTransferFormationToMain DraftScenarioStepType = "transfer_formation_to_main"
)

// DraftScenario описывает черновой результат работы эвристики.
//
// Это уже не просто набор решений и операций, а упорядоченный draft plan,
// привязанный к конкретной схеме и целевому цвету. Однако сценарий всё ещё
// остаётся высокоуровневым: он не содержит конкретных low-level шагов движения.
//
// Значение полей:
//   - SchemeID: идентификатор схемы, для которой построен draft scenario
//   - TargetColor: целевой цвет, который требуется собрать
//   - RequiredTargetCount: требуемое количество целевых вагонов K
//   - FormationTrackID: путь формирования целевого состава
//   - BufferTrackID: буферный путь для временного размещения blocking-вагонов
//   - MainTrackID: главный путь, на который в конце должен перейти состав
//   - Steps: упорядоченный список черновых шагов сценария
//
// Ожидаемые инварианты:
//   - Steps отсортированы по StepOrder
//   - все идентификаторы путей взяты из FixedClassProblem
//   - сценарий остаётся декларативным и не предназначен для прямого исполнения
type DraftScenario struct {
	SchemeID            int
	TargetColor         string
	RequiredTargetCount int
	FormationTrackID    string
	BufferTrackID       string
	MainTrackID         string
	Steps               []DraftScenarioStep
}

// DraftScenarioStep описывает один шаг чернового эвристического сценария.
//
// Структура хранит достаточно данных, чтобы следующий этап мог преобразовать
// draft шаг в более конкретные шаги построения сценария, но при этом не
// фиксирует никаких деталей low-level исполнения.
//
// Значение полей:
//   - StepOrder: порядковый номер шага в черновом сценарии
//   - StepType: тип draft-шага
//   - SourceTrackID: путь-источник действия
//   - DestinationTrackID: путь-назначение действия
//   - SourceSide: сторона исходного пути, если она имеет значение
//   - WagonCount: количество вагонов, участвующих в шаге
//   - TargetColor: целевой цвет, к которому относится шаг
//   - FormationTrackID: путь формирования
//   - BufferTrackID: буферный путь
//   - MainTrackID: главный путь
//
// Ожидаемые инварианты:
//   - StepOrder уникален внутри одного DraftScenario
//   - SourceTrackID и DestinationTrackID заполнены для поддержанных типов шагов
//   - WagonCount >= 0
//   - шаг является доменным описанием, а не исполнимой low-level командой
type DraftScenarioStep struct {
	StepOrder          int
	StepType           DraftScenarioStepType
	SourceTrackID      string
	DestinationTrackID string
	SourceSide         string
	WagonCount         int
	TargetColor        string
	FormationTrackID   string
	BufferTrackID      string
	MainTrackID        string
}

// BuildDraftScenario строит черновой эвристический сценарий на основе
// промежуточных доменных операций.
//
// Входные данные:
//   - problem: валидированное fixed-class описание задачи; из него берутся
//     общие параметры draft scenario
//   - operations: ordered список доменных операций, который нужно представить
//     в виде draft scenario
//
// Правила построения:
//   - каждая операция превращается в один DraftScenarioStep
//   - порядок операций сохраняется как StepOrder
//   - тип операции напрямую отображается в соответствующий тип draft-шага
//
// Ограничения:
//   - функция не строит normalized scenario_steps
//   - функция не добавляет low-level movement details
//   - функция формирует только draft-представление будущего сценария
func BuildDraftScenario(problem FixedClassProblem, operations []HeuristicOperation) DraftScenario {
	scenario := DraftScenario{
		SchemeID:            problem.SchemeID,
		TargetColor:         problem.TargetColor,
		RequiredTargetCount: len(problem.TargetWagons),
		FormationTrackID:    problem.FormationTrack.TrackID,
		BufferTrackID:       problem.BufferTrack.TrackID,
		MainTrackID:         problem.MainTrack.TrackID,
		Steps:               make([]DraftScenarioStep, 0, len(operations)),
	}

	for index, operation := range operations {
		step := DraftScenarioStep{
			StepOrder:          index,
			SourceTrackID:      operation.SourceTrackID,
			DestinationTrackID: operation.DestinationTrackID,
			SourceSide:         operation.SourceSide,
			WagonCount:         operation.WagonCount,
			TargetColor:        operation.TargetColor,
			FormationTrackID:   operation.FormationTrackID,
			BufferTrackID:      operation.BufferTrackID,
			MainTrackID:        operation.MainTrackID,
		}

		switch operation.OperationType {
		case HeuristicOperationBufferBlockers:
			step.StepType = DraftScenarioStepBufferBlockers
		case HeuristicOperationTransferTargetsToFormation:
			step.StepType = DraftScenarioStepTransferTargetsToFormation
		case HeuristicOperationTransferFormationToMain:
			step.StepType = DraftScenarioStepTransferFormationToMain
		}

		scenario.Steps = append(scenario.Steps, step)
	}

	return scenario
}
