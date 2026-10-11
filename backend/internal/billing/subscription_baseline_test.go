package billing_test

import (
	"context"
	"testing"
	"time"

	"pressing-os/backend/internal/billing"
)

// These tests characterize the existing subscription engine before changing
// quota periods or expiry behavior. Keep them as regression guards.
func TestSubscriptionBaseline_TrialIsSevenDaysAndIdempotent(t *testing.T) {
	ctx := context.Background()
	svc := billing.NewService()
	tenant := "61000000-0000-4000-8000-000000000001"

	if err := svc.EnsureTrial(ctx, tenant); err != nil {
		t.Fatalf("EnsureTrial first call: %v", err)
	}
	first, err := svc.GetSubscription(ctx, tenant)
	if err != nil || first == nil {
		t.Fatalf("GetSubscription after first trial: record=%+v err=%v", first, err)
	}
	if first.Status != billing.StatusTrialing {
		t.Fatalf("status = %q, want %q", first.Status, billing.StatusTrialing)
	}
	if first.TrialStartedAt == nil || first.TrialEndsAt == nil {
		t.Fatal("trial start/end timestamps must be persisted")
	}
	if got := first.TrialEndsAt.Sub(*first.TrialStartedAt); got < 7*24*time.Hour-time.Second || got > 7*24*time.Hour+time.Second {
		t.Fatalf("trial duration = %s, want 7 days", got)
	}

	if err := svc.EnsureTrial(ctx, tenant); err != nil {
		t.Fatalf("EnsureTrial second call: %v", err)
	}
	second, err := svc.GetSubscription(ctx, tenant)
	if err != nil || second == nil {
		t.Fatalf("GetSubscription after second trial: record=%+v err=%v", second, err)
	}
	if !second.TrialStartedAt.Equal(*first.TrialStartedAt) || !second.TrialEndsAt.Equal(*first.TrialEndsAt) {
		t.Fatalf("second EnsureTrial must not reset the trial window: first=%+v second=%+v", first, second)
	}
}

func TestSubscriptionBaseline_TrialUsesBusinessEntitlements(t *testing.T) {
	ctx := context.Background()
	svc := billing.NewService()
	tenant := "61000000-0000-4000-8000-000000000002"

	if err := svc.EnsureTrial(ctx, tenant); err != nil {
		t.Fatalf("EnsureTrial: %v", err)
	}
	resp, err := svc.EffectiveEntitlements(ctx, tenant)
	if err != nil {
		t.Fatalf("EffectiveEntitlements: %v", err)
	}
	if !resp.OnTrial || resp.PlanCode != billing.PlanBusiness {
		t.Fatalf("trial effective state = on_trial:%t plan:%s, want true/BUSINESS", resp.OnTrial, resp.PlanCode)
	}
	if resp.TrialEndsAt == nil || resp.DaysLeft == nil {
		t.Fatal("trial response must expose end date and days remaining")
	}
}

func TestSubscriptionBaseline_PlanAndQuotaAreTenantScoped(t *testing.T) {
	ctx := context.Background()
	svc := billing.NewService()
	trialTenant := "61000000-0000-4000-8000-000000000003"
	freeTenant := "61000000-0000-4000-8000-000000000004"

	if err := svc.EnsureTrial(ctx, trialTenant); err != nil {
		t.Fatalf("EnsureTrial: %v", err)
	}
	svc.ForcePlanForTest(freeTenant, billing.PlanFree, billing.StatusActive)

	trialEnabled, err := svc.IsEnabled(ctx, trialTenant, billing.FeatureReportsAdvanced)
	if err != nil || !trialEnabled {
		t.Fatalf("trial reports.advanced: enabled=%t err=%v, want enabled", trialEnabled, err)
	}
	freeEnabled, err := svc.IsEnabled(ctx, freeTenant, billing.FeatureReportsAdvanced)
	if err != nil {
		t.Fatalf("free reports.advanced: %v", err)
	}
	if freeEnabled {
		t.Fatal("FREE tenant must not inherit the trial tenant's advanced-report entitlement")
	}

	if err := svc.CheckLimit(ctx, freeTenant, billing.FeatureOrdersMonthlyMax, 49); err != nil {
		t.Fatalf("FREE quota must allow order 50: %v", err)
	}
	if err := svc.CheckLimit(ctx, freeTenant, billing.FeatureOrdersMonthlyMax, 50); err == nil {
		t.Fatal("FREE quota must reject order 51")
	}
	if err := svc.CheckLimit(ctx, trialTenant, billing.FeatureOrdersMonthlyMax, 500); err != nil {
		t.Fatalf("active BUSINESS-level trial should not have the FREE 50-order cap: %v", err)
	}
}
