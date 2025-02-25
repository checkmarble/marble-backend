package usecases

type VersionUsecase struct {
	ApiVersion string
}

func (uc VersionUsecase) GetApiVersion() string {
	return uc.ApiVersion
}
