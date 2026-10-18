package pages

import (
	"context"
	"panlyackerp/internal/platform/locale"
)

func T(ctx context.Context, source string) string { return locale.Text(ctx, source) }

func PlanPrice(ctx context.Context, amount string) string {
	if locale.Language(ctx) == "en" {
		return "$" + amount
	}
	return amount + " $"
}
