package config

type Store struct {
	Path InterpolatedString `yaml:"path"`
}

func NewDefaultStoreConfig() Store {
	return Store{
		Path: "${CALLI_STORE_PATH:-data.db}",
	}
}
