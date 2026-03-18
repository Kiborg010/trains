package heuristic

import (
	"fmt"
	"math"

	"trains/backend/normalized"
)

// Этот файл реализует ШАГ 7 первой backend-эвристики.
//
// Его задача — добавить поверх draft heuristic scenario слой оценки стоимости
// и выполнимости. На этом этапе сценарий всё ещё остаётся draft-представлением,
// но теперь каждый шаг и весь сценарий целиком можно оценить по упрощённой
// операционной модели.
//
// Важно:
//   - здесь НЕ генерируются low-level move_loco / couple / decouple
//   - здесь НЕ создаются normalized scenario_steps
//   - здесь НЕ выполняется execution integration
//   - здесь НЕ строится точная кинематика локомотива
//
// Что оценивается:
//   - число сцепок и расцепок
//   - условная длина маршрута локомотива/операции
//   - число пройденных стрелок
//   - суммарная стоимость шага и сценария
//   - выполнимость шага с точки зрения доступности маршрута
//
// Source of truth для маршрута:
//   - normalized tracks
//   - normalized track_connections
//
// Маршрут считается по графу путей. Геометрия используется только для оценки
// длины пути, а не как источник топологии.

// DraftStepCost описывает оценку одного draft-шага сценария.
//
// Значение полей:
//   - CoupleCount: число сцепок, которое закладывается в шаг
//   - DecoupleCount: число расцепок, которое закладывается в шаг
//   - LocoDistance: упрощённая оценка длины маршрута по путям
//   - SwitchCrossCount: число стрелочных соединений на найденном маршруте
//   - TotalCost: суммарная стоимость шага по текущей простой формуле
//   - Feasible: можно ли считать шаг допустимым на уровне draft-оценки
//   - Reasons: пояснения о причинах недопустимости или важных ограничениях шага
//
// Ожидаемые инварианты:
//   - CoupleCount и DecoupleCount неотрицательны
//   - LocoDistance неотрицательно
//   - SwitchCrossCount неотрицателен
//   - если Feasible == false, Reasons должен быть непустым
type DraftStepCost struct {
	CoupleCount      int
	DecoupleCount    int
	LocoDistance     float64
	SwitchCrossCount int
	TotalCost        float64
	Feasible         bool
	Reasons          []string
}

// DraftScenarioMetrics описывает агрегированную оценку всего draft-сценария.
//
// Значение полей:
//   - TotalStepCount: общее число шагов в draft-сценарии
//   - TotalCoupleCount: суммарное число сцепок
//   - TotalDecoupleCount: суммарное число расцепок
//   - TotalLocoDistance: суммарная длина маршрутов по всем шагам
//   - TotalSwitchCrossCount: суммарное число пройденных стрелок
//   - TotalCost: итоговая стоимость по всем шагам
//   - Success: общий флаг допустимости сценария
//
// Ожидаемые инварианты:
//   - агрегаты равны сумме соответствующих полей по всем шагам
//   - Success == true только если все шаги признаны допустимыми
type DraftScenarioMetrics struct {
	TotalStepCount        int
	TotalCoupleCount      int
	TotalDecoupleCount    int
	TotalLocoDistance     float64
	TotalSwitchCrossCount int
	TotalCost             float64
	Success               bool
}

type trackRouteNode struct {
	Distance    float64
	TrackID     string
	SwitchCount int
	Path        []string
}

type trackNeighbor struct {
	TrackID        string
	ConnectionType string
}

// EvaluateDraftScenarioStepCost оценивает стоимость и допустимость одного
// draft-шага сценария.
//
// Правила оценки:
//   - на каждый шаг закладывается 1 сцепка и 1 расцепка
//   - маршрут между source и destination ищется по track_connections
//   - длина маршрута считается как сумма длин путей вдоль найденного маршрута
//   - каждое соединение типа switch увеличивает SwitchCrossCount
//
// Важное ограничение на прохождение стрелок:
//   - если шаг проводит группу вагонов по маршруту, содержащему стрелки,
//     то вся группа рассматривается как единый перенос внутри этого draft-шага
//   - частичное проведение группы через стрелочный маршрут внутри одного
//     draft-шага не допускается
//
// Это ограничение сейчас отражается так:
//   - шаг остаётся допустимым, если сам маршрут найден
//   - в Reasons явно добавляется пояснение, что группа проходит стрелочный
//     маршрут только целиком
func EvaluateDraftScenarioStepCost(problem FixedClassProblem, step DraftScenarioStep) DraftStepCost {
	cost := DraftStepCost{
		CoupleCount:   1,
		DecoupleCount: 1,
		Feasible:      false,
		Reasons:       []string{},
	}

	if step.SourceTrackID == "" {
		cost.Reasons = append(cost.Reasons, "source track id is required")
		return cost
	}
	if step.DestinationTrackID == "" {
		cost.Reasons = append(cost.Reasons, "destination track id is required")
		return cost
	}
	if step.WagonCount <= 0 {
		cost.Reasons = append(cost.Reasons, "wagon count must be positive")
		return cost
	}
	if _, ok := problem.TracksByID[step.SourceTrackID]; !ok {
		cost.Reasons = append(cost.Reasons, fmt.Sprintf("source track %s was not found", step.SourceTrackID))
		return cost
	}
	if _, ok := problem.TracksByID[step.DestinationTrackID]; !ok {
		cost.Reasons = append(cost.Reasons, fmt.Sprintf("destination track %s was not found", step.DestinationTrackID))
		return cost
	}

	route, switchCrossCount, routeFound := findShortestTrackRoute(problem, step.SourceTrackID, step.DestinationTrackID)
	if !routeFound {
		cost.Reasons = append(cost.Reasons, fmt.Sprintf(
			"route was not found between %s and %s using track_connections",
			step.SourceTrackID,
			step.DestinationTrackID,
		))
		return cost
	}

	cost.LocoDistance = routeLength(problem, route)
	cost.SwitchCrossCount = switchCrossCount

	if switchCrossCount > 0 {
		cost.Reasons = append(cost.Reasons,
			"the wagon group must traverse the switch route as a whole; partial split inside one draft step is not allowed",
		)
	}

	cost.TotalCost = float64(cost.CoupleCount+cost.DecoupleCount) + cost.LocoDistance + float64(cost.SwitchCrossCount)
	cost.Feasible = true
	return cost
}

// EvaluateDraftScenarioMetrics оценивает весь draft-сценарий целиком,
// суммируя стоимости его шагов.
//
// Сценарий считается успешным только если каждый шаг допустим.
// Если хотя бы один шаг недопустим, Success становится false, но уже
// посчитанные агрегаты всё равно возвращаются, чтобы сценарий можно было
// диагностировать и сравнивать с альтернативами.
func EvaluateDraftScenarioMetrics(problem FixedClassProblem, scenario DraftScenario) DraftScenarioMetrics {
	metrics := DraftScenarioMetrics{
		TotalStepCount: len(scenario.Steps),
		Success:        true,
	}

	for _, step := range scenario.Steps {
		cost := EvaluateDraftScenarioStepCost(problem, step)
		metrics.TotalCoupleCount += cost.CoupleCount
		metrics.TotalDecoupleCount += cost.DecoupleCount
		metrics.TotalLocoDistance += cost.LocoDistance
		metrics.TotalSwitchCrossCount += cost.SwitchCrossCount
		metrics.TotalCost += cost.TotalCost
		if !cost.Feasible {
			metrics.Success = false
		}
	}

	return metrics
}

func findShortestTrackRoute(problem FixedClassProblem, sourceTrackID string, destinationTrackID string) ([]string, int, bool) {
	if sourceTrackID == destinationTrackID {
		return []string{sourceTrackID}, 0, true
	}

	adjacency := buildTrackConnectionAdjacency(problem.TrackConnections)
	if len(adjacency) == 0 {
		return nil, 0, false
	}

	bestDistance := map[string]float64{
		sourceTrackID: trackLength(problem.TracksByID[sourceTrackID]),
	}
	bestSwitchCount := map[string]int{
		sourceTrackID: 0,
	}
	queue := []trackRouteNode{{
		Distance:    bestDistance[sourceTrackID],
		TrackID:     sourceTrackID,
		SwitchCount: 0,
		Path:        []string{sourceTrackID},
	}}

	for len(queue) > 0 {
		sortTrackRouteNodes(queue)
		current := queue[0]
		queue = queue[1:]

		if current.TrackID == destinationTrackID {
			return current.Path, current.SwitchCount, true
		}

		for _, neighbor := range adjacency[current.TrackID] {
			track, ok := problem.TracksByID[neighbor.TrackID]
			if !ok {
				continue
			}

			nextDistance := current.Distance + trackLength(track)
			nextSwitchCount := current.SwitchCount
			if neighbor.ConnectionType == "switch" {
				nextSwitchCount++
			}

			existingDistance, seen := bestDistance[neighbor.TrackID]
			existingSwitchCount := bestSwitchCount[neighbor.TrackID]
			if seen && (existingDistance < nextDistance || (existingDistance == nextDistance && existingSwitchCount <= nextSwitchCount)) {
				continue
			}

			bestDistance[neighbor.TrackID] = nextDistance
			bestSwitchCount[neighbor.TrackID] = nextSwitchCount
			nextPath := append(append([]string{}, current.Path...), neighbor.TrackID)
			queue = append(queue, trackRouteNode{
				Distance:    nextDistance,
				TrackID:     neighbor.TrackID,
				SwitchCount: nextSwitchCount,
				Path:        nextPath,
			})
		}
	}

	return nil, 0, false
}

func buildTrackConnectionAdjacency(connections []normalized.TrackConnection) map[string][]trackNeighbor {
	adjacency := make(map[string][]trackNeighbor)
	for _, connection := range connections {
		adjacency[connection.Track1ID] = append(adjacency[connection.Track1ID], trackNeighbor{
			TrackID:        connection.Track2ID,
			ConnectionType: connection.ConnectionType,
		})
		adjacency[connection.Track2ID] = append(adjacency[connection.Track2ID], trackNeighbor{
			TrackID:        connection.Track1ID,
			ConnectionType: connection.ConnectionType,
		})
	}
	return adjacency
}

func sortTrackRouteNodes(nodes []trackRouteNode) {
	for i := 0; i < len(nodes)-1; i++ {
		best := i
		for j := i + 1; j < len(nodes); j++ {
			if nodes[j].Distance < nodes[best].Distance ||
				(nodes[j].Distance == nodes[best].Distance && nodes[j].SwitchCount < nodes[best].SwitchCount) ||
				(nodes[j].Distance == nodes[best].Distance && nodes[j].SwitchCount == nodes[best].SwitchCount && nodes[j].TrackID < nodes[best].TrackID) {
				best = j
			}
		}
		nodes[i], nodes[best] = nodes[best], nodes[i]
	}
}

func routeLength(problem FixedClassProblem, route []string) float64 {
	total := 0.0
	for _, trackID := range route {
		track, ok := problem.TracksByID[trackID]
		if !ok {
			continue
		}
		total += trackLength(track)
	}
	return total
}

func trackLength(track normalized.Track) float64 {
	return math.Hypot(track.EndX-track.StartX, track.EndY-track.StartY)
}
