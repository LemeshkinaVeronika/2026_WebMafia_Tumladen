package service

type Engine struct {
	catalog TileCatalog
}

func NewEngine() (*Engine, error) {
	catalog, err := NewAssetTileCatalog()
	if err != nil {
		return nil, err
	}

	return &Engine{
		catalog: catalog,
	}, nil
}

func (e *Engine) GameType() string {
	return "carcassonne"
}
