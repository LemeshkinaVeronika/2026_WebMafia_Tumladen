package service

import (
	"encoding/json"

	carcassonneDTO "github.com/webmafia/tumladan/internal/game/carcassonne/dto"
)

func (e *Engine) BuildTileCatalog() (json.RawMessage, error) {
	definitions := e.catalog.Definitions()
	items := make([]carcassonneDTO.TileCatalogItem, 0, len(definitions))

	for _, def := range definitions {
		segments := make([]carcassonneDTO.ZoneSegment, 0)
		hasPennant := false

		for _, zone := range def.Zones {
			segments = append(segments, zone.Segments...)
			if zone.HasPennant {
				hasPennant = true
			}
		}

		items = append(items, carcassonneDTO.TileCatalogItem{
			TileID:     def.TileID,
			ImageURL:   def.ImageKey,
			Edges:      def.Edges,
			Zones:      def.Zones,
			Segments:   segments,
			HasPennant: hasPennant,
			Count:      def.Count,
		})
	}

	data, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}

	return data, nil
}
