package grantan

import "time"

type aiDecision struct {
	kind  string
	build string
	give  Resource
	get   Resource
}

func (s *Server) ensureAITurn(roomID string) {
	room := s.getRoom(roomID)
	if room == nil {
		return
	}

	room.mu.Lock()
	if room.aiRunning || !room.started || room.phase == phaseFinished {
		room.mu.Unlock()
		return
	}
	current := room.currentPlayerLocked()
	if current == nil || !current.IsAI {
		room.mu.Unlock()
		return
	}
	room.aiRunning = true
	room.mu.Unlock()

	go s.runAI(roomID)
}

// runAI is the delayed CPU loop. The logic is intentionally lightweight so AI
// turns feel human-sized and the behavior stays easy to reason about.
func (s *Server) runAI(roomID string) {
	for {
		time.Sleep(randomAIDelay())

		room := s.getRoom(roomID)
		if room == nil {
			return
		}

		room.mu.Lock()
		if !room.started || room.phase == phaseFinished {
			room.aiRunning = false
			room.mu.Unlock()
			return
		}

		player := room.currentPlayerLocked()
		if player == nil || !player.IsAI {
			room.aiRunning = false
			room.mu.Unlock()
			return
		}

		var err error
		if room.phase == phaseRoll {
			err = room.rollLocked(player.ID)
		} else {
			decision := chooseAIAction(player)
			switch decision.kind {
			case "build":
				err = room.buildLocked(player.ID, decision.build)
			case "trade":
				err = room.tradeLocked(player.ID, decision.give, decision.get)
			default:
				err = room.endTurnLocked(player.ID)
			}
		}

		state := room.stateLocked()
		clients := room.clientsLocked()
		continueLoop := room.started && room.phase != phaseFinished && room.currentPlayerLocked() != nil && room.currentPlayerLocked().IsAI
		if !continueLoop {
			room.aiRunning = false
		}
		room.mu.Unlock()

		if err == nil {
			broadcastState(clients, state)
		}
		if err != nil || !continueLoop {
			return
		}
	}
}

func chooseAIAction(player *Player) aiDecision {
	if canAfford(player, buildCosts["city"]) && player.Settlements > 0 {
		return aiDecision{kind: "build", build: "city"}
	}
	if canAfford(player, buildCosts["settlement"]) && player.Roads >= player.Settlements+player.Cities {
		return aiDecision{kind: "build", build: "settlement"}
	}
	if trade := tradeForTarget(player, buildCosts["city"]); trade.kind != "" {
		return trade
	}
	if trade := tradeForTarget(player, buildCosts["settlement"]); trade.kind != "" {
		return trade
	}
	if canAfford(player, buildCosts["road"]) && player.Roads < 15 {
		return aiDecision{kind: "build", build: "road"}
	}
	return aiDecision{kind: "end"}
}

func tradeForTarget(player *Player, cost map[Resource]int) aiDecision {
	for _, wanted := range resourceOrder {
		need := cost[wanted] - player.Resources[wanted]
		if need <= 0 {
			continue
		}
		for _, resource := range resourceOrder {
			if resource == wanted {
				continue
			}
			if player.Resources[resource]-cost[resource] >= 4 {
				return aiDecision{
					kind: "trade",
					give: resource,
					get:  wanted,
				}
			}
		}
	}
	return aiDecision{}
}
