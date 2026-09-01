package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/farhapartex/coyote/core/auth"
)

const NameCreateSuperadmin = "createsuperadmin"

var ErrSchemaMissing = errors.New("coyote/cli: the users table does not exist yet")

type CreateSuperadmin struct {
	Username string
	Email    string
	Password string
}

func (CreateSuperadmin) Name() string { return NameCreateSuperadmin }

func (CreateSuperadmin) Summary() string {
	return "create a superadmin who can sign in to the admin portal"
}

func (c CreateSuperadmin) Run(ctx Context) error {
	if err := c.requireSchema(ctx); err != nil {
		return err
	}

	reader := bufio.NewReader(ctx.Input())
	username, err := c.resolve(ctx, reader, "Username", c.Username, EnvUsername, true)
	if err != nil {
		return err
	}
	email, err := c.resolve(ctx, reader, "Email", c.Email, EnvEmail, false)
	if err != nil {
		return err
	}
	password, err := c.resolveSecret(ctx, reader, "Password", c.Password, EnvPassword)
	if err != nil {
		return err
	}

	user, err := ctx.App.AuthService().CreateSuperadmin(ctx.Context(), username, email, password)
	if err != nil {
		return err
	}
	fmt.Fprintf(ctx.Out, "\ncreated superadmin %s (%s)\n", user.Username, user.ID)
	fmt.Fprintf(ctx.Out, "sign in at %s%s/login\n", ctx.App.Config().Addr(), ctx.App.Config().Admin.Prefix)
	return nil
}

func (c CreateSuperadmin) requireSchema(ctx Context) error {
	handle, err := ctx.App.DB()
	if err != nil {
		return err
	}
	if !handle.Migrator().HasTable(&auth.User{}) {
		return fmt.Errorf("%w; run: coyote makemigrations && coyote migrate", ErrSchemaMissing)
	}
	return nil
}

func (c CreateSuperadmin) resolveSecret(ctx Context, reader *bufio.Reader, label, given, env string) (string, error) {
	if value := strings.TrimSpace(given); value != "" {
		return value, nil
	}
	if value := strings.TrimSpace(os.Getenv(env)); value != "" {
		return value, nil
	}
	if value, handled, err := ctx.ReadSecret(label); handled {
		if err != nil {
			return "", err
		}
		if value == "" {
			return "", fmt.Errorf("coyote/cli: %s is required", strings.ToLower(label))
		}
		return value, nil
	}
	return c.resolve(ctx, reader, label, given, env, true)
}

func (c CreateSuperadmin) resolve(ctx Context, reader *bufio.Reader, label, given, env string, required bool) (string, error) {
	if value := strings.TrimSpace(given); value != "" {
		return value, nil
	}
	if value := strings.TrimSpace(os.Getenv(env)); value != "" {
		return value, nil
	}
	for {
		fmt.Fprintf(ctx.Out, "%s: ", label)
		line, err := reader.ReadString('\n')
		value := strings.TrimSpace(line)
		if value != "" {
			return value, nil
		}
		if err != nil {
			if required {
				return "", fmt.Errorf("coyote/cli: %s is required", strings.ToLower(label))
			}
			return "", nil
		}
		if !required {
			return "", nil
		}
		fmt.Fprintf(ctx.Out, "%s cannot be blank\n", label)
	}
}
