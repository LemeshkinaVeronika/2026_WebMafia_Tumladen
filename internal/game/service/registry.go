package service

import "fmt"

type Registry struct {
	engines map[string]Engine
}

func NewRegistry(engines ...Engine) (*Registry, error) {
	items := make(map[string]Engine, len(engines))

	for _, engine := range engines {
		if engine == nil {
			return nil, fmt.Errorf("nil game engine")
		}

		gameType := engine.GameType()
		if gameType == "" {
			return nil, fmt.Errorf("empty game type")
		}

		if _, exists := items[gameType]; exists {
			return nil, fmt.Errorf("duplicate game engine: %s", gameType)
		}

		items[gameType] = engine
	}

	return &Registry{
		engines: items,
	}, nil
}

func (r *Registry) Get(gameType string) (Engine, error) {
	engine, ok := r.engines[gameType]
	if !ok {
		return nil, ErrUnsupportedGameType
	}

	return engine, nil
}
