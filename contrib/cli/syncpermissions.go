package cli

import (
	"errors"
	"fmt"
)

type SyncPermissions struct{}

func (SyncPermissions) Name() string { return NameSyncPermissions }

func (SyncPermissions) Summary() string { return "create the permissions for every registered model" }

func (SyncPermissions) Run(ctx Context) error {
	report, err := syncPermissions(ctx)
	if err != nil {
		return err
	}
	reportPermissions(ctx, report)
	return nil
}

func syncPermissions(ctx Context) (Report, error) {
	syncer, ok := ctx.App.(PermissionSyncer)
	if !ok {
		return Report{}, errors.New("coyote/cli: this application cannot sync permissions")
	}
	report, err := syncer.SyncPermissions()
	if err != nil {
		return Report{}, err
	}
	return Report{Created: report.Created, Stale: report.Stale}, nil
}

func reportPermissions(ctx Context, report Report) {
	if len(report.Created) == 0 && len(report.Stale) == 0 {
		fmt.Fprintln(ctx.Out, "permissions are up to date")
		return
	}
	for _, codename := range report.Created {
		fmt.Fprintf(ctx.Out, "  + %s\n", codename)
	}
	if len(report.Created) > 0 {
		fmt.Fprintf(ctx.Out, "\ncreated %d permission(s)\n", len(report.Created))
	}
	if len(report.Stale) > 0 {
		fmt.Fprintf(ctx.Out, "\n%d permission(s) no longer match a model; review before removing them:\n", len(report.Stale))
		for _, codename := range report.Stale {
			fmt.Fprintf(ctx.Out, "  ! %s\n", codename)
		}
	}
}
