package grantan

import (
	"crypto/rand"
	"encoding/hex"
	mathrand "math/rand"
	"time"
)

var aiNames = []string{
	"Maple Bot",
	"Copper Bot",
	"Harbor Bot",
}

func randomID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("15040500")))
	}
	return hex.EncodeToString(bytes)
}

func randomCode(length int) string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "GAME"
	}
	for i := range bytes {
		bytes[i] = alphabet[int(bytes[i])%len(alphabet)]
	}
	return string(bytes)
}

func rollDice() int {
	return mathrand.Intn(6) + 1 + mathrand.Intn(6) + 1
}

func randomResource() Resource {
	return resourceOrder[mathrand.Intn(len(resourceOrder))]
}

func removeRandomResource(player *Player) Resource {
	available := make([]Resource, 0, len(resourceOrder))
	for _, resource := range resourceOrder {
		if player.Resources[resource] > 0 {
			available = append(available, resource)
		}
	}
	if len(available) == 0 {
		return ""
	}
	resource := available[mathrand.Intn(len(available))]
	player.Resources[resource]--
	return resource
}

func aiName(index int) string {
	if index < len(aiNames) {
		return aiNames[index]
	}
	return "Grantan Bot"
}

func randomAIDelay() time.Duration {
	return time.Duration(1000+mathrand.Intn(1000)) * time.Millisecond
}

func canAfford(player *Player, cost map[Resource]int) bool {
	for resource, amount := range cost {
		if player.Resources[resource] < amount {
			return false
		}
	}
	return true
}

func payCost(player *Player, cost map[Resource]int) {
	for resource, amount := range cost {
		player.Resources[resource] -= amount
	}
}
