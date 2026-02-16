package evaluate

import (
	"context"
	"slices"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/usecases/payload_parser"
)

var validIpFlags = []string{
	"abuse",
	"tor_exit_node",
	"vpn",
	"cloud_provider",
}

type HasIpFlag struct {
	PayloadEnricher payload_parser.PayloadEnrichementUsecase
}

func (f HasIpFlag) Evaluate(ctx context.Context, args ast.Arguments) (any, []error) {
	if args.NamedArgs["ip"] == nil {
		return nil, nil
	}

	var errs []error

	ip, ipErr := AdaptNamedArgument(args.NamedArgs, "ip", adaptArgumentToIp)
	flag, flagErr := AdaptNamedArgument(args.NamedArgs, "flag", adaptArgumentToString)

	errs = append(errs, ipErr)
	errs = append(errs, flagErr)

	if flagErr == nil && !slices.Contains(validIpFlags, flag) {
		errs = append(errs, ast.NewNamedArgumentError("flag"))
	}

	errs = filterNilErrors(errs...)
	if len(errs) > 0 {
		return nil, errs
	}

	metadata := f.PayloadEnricher.EnrichIp(ip)

	if metadata == nil {
		return false, nil
	}

	switch flag {
	case "abuse":
		return metadata.Abuse, nil
	case "cloud_provider":
		return metadata.CloudProvider, nil
	case "vpn":
		return metadata.Vpn, nil
	case "tor_exit_node":
		return metadata.TorExitNode, nil
	}

	return nil, nil
}
