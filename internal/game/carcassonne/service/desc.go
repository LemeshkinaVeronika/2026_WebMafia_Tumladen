package service

import (
	"math/rand"
	"time"

	"github.com/google/uuid"
	carcassonneDTO "github.com/webmafia/tumladan/internal/game/carcassonne/dto"
)

func buildDeck(tileIDs []string) []carcassonneDTO.TileInstance {
	deck := make([]carcassonneDTO.TileInstance, 0, len(tileIDs))
	for _, tileID := range tileIDs {
		deck = append(deck, carcassonneDTO.TileInstance{
			InstanceID: uuid.NewString(),
			TileID:     tileID,
		})
	}
	return deck
}

func shuffleDeck(deck []carcassonneDTO.TileInstance) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})
}

func drawTopTile(deck []carcassonneDTO.TileInstance) (*carcassonneDTO.TileInstance, []carcassonneDTO.TileInstance) {
	if len(deck) == 0 {
		return nil, deck
	}

	tile := deck[0]
	rest := deck[1:]
	return &tile, rest
}
