package config

import "time"

// Payment is the top-up framework's process configuration (internal/payment). Kept separate from Config so the
// payment wiring stays self-contained.
type Payment struct {
	// SandboxSecret signs/verifies sandbox webhooks (HMAC-SHA256, ≥ 32 bytes). Empty in dev → random per process.
	// Required wherever the sandbox provider is enabled outside dev (payments.yaml envs). Never logged.
	SandboxSecret string
	// SweepInterval is how often the worker expires stale orders and reconciles pending ones.
	SweepInterval time.Duration
}

// LoadPayment reads PAYMENT_* env.
func LoadPayment() Payment {
	return Payment{
		SandboxSecret: get("PAYMENT_SANDBOX_SECRET", ""),
		SweepInterval: time.Duration(getInt("PAYMENT_SWEEP_SECONDS", 60)) * time.Second,
	}
}
