package ai_agent

import (
	"encoding/json"
	"io"
	"io/fs"

	"github.com/checkmarble/marble-backend/models"
	"github.com/pkg/errors"
)

type RuleCatalogUsecase struct {
	aiPromptFs fs.FS
}

func NewRuleCatalogUsecase(aiPromptFs fs.FS) RuleCatalogUsecase {
	return RuleCatalogUsecase{
		aiPromptFs: aiPromptFs,
	}
}

func (uc RuleCatalogUsecase) GetRuleCatalog() (models.RuleCatalog, error) {
	f, err := uc.aiPromptFs.Open("rule_catalog.json")
	if err != nil {
		return models.RuleCatalog{}, errors.Wrap(err, "could not open rule catalog file")
	}
	defer f.Close()

	var catalog models.RuleCatalog

	b, err := io.ReadAll(f)
	if err != nil {
		return models.RuleCatalog{}, errors.Wrap(err, "could not read rule catalog file")
	}

	if err := json.Unmarshal(b, &catalog); err != nil {
		return models.RuleCatalog{}, errors.Wrap(err, "could not unmarshal rule catalog file")
	}

	return catalog, nil
}
