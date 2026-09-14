package domain

// State is a durable recovery state. Transitions are explicit so retries and
// restarts cannot skip policy authorization or final verification.
type State string

const (
	StateDetected           State = "detected"
	StateFrozen             State = "frozen"
	StateInvestigating      State = "investigating"
	StateRecoveryPlanned    State = "recovery_planned"
	StatePolicyAuthorized   State = "policy_authorized"
	StateRetrying           State = "retrying"
	StateMitigating         State = "mitigating"
	StateMitigationVerified State = "mitigation_verified"
	StateQuarantining       State = "quarantining"
	StateReconcilingGit     State = "reconciling_git"
	StateApplyingRecovery   State = "applying_recovery"
	StateFinalVerification  State = "final_verification"
	StateRecovered          State = "recovered"
	StateMitigatedWithDrift State = "mitigated_with_drift"
	StateSuperseded         State = "superseded"
	StateCircuitOpen        State = "circuit_open"
	StateRecoveryFailed     State = "recovery_failed"
	StateEscalated          State = "escalated"
)

func (s State) Valid() bool {
	switch s {
	case StateDetected, StateFrozen, StateInvestigating, StateRecoveryPlanned,
		StatePolicyAuthorized, StateRetrying, StateMitigating, StateMitigationVerified,
		StateQuarantining, StateReconcilingGit, StateApplyingRecovery,
		StateFinalVerification, StateRecovered, StateMitigatedWithDrift,
		StateSuperseded, StateCircuitOpen, StateRecoveryFailed, StateEscalated:
		return true
	default:
		return false
	}
}

func CanTransition(from, to State) bool {
	switch from {
	case StateDetected:
		return oneOf(to, StateFrozen, StateSuperseded, StateEscalated)
	case StateFrozen:
		return oneOf(to, StateInvestigating, StateSuperseded, StateEscalated)
	case StateInvestigating:
		return oneOf(to, StateRecoveryPlanned, StateRecoveryFailed, StateSuperseded, StateEscalated)
	case StateRecoveryPlanned:
		return oneOf(to, StatePolicyAuthorized, StateRecoveryFailed, StateSuperseded, StateEscalated)
	case StatePolicyAuthorized:
		return oneOf(to, StateRetrying, StateMitigating, StateQuarantining, StateRecoveryFailed, StateSuperseded)
	case StateRetrying:
		return oneOf(to, StateInvestigating, StateFinalVerification, StateRecoveryFailed, StateCircuitOpen, StateSuperseded)
	case StateMitigating:
		return oneOf(to, StateMitigationVerified, StateRecoveryFailed, StateCircuitOpen, StateSuperseded)
	case StateMitigationVerified:
		return oneOf(to, StateQuarantining, StateReconcilingGit, StateFinalVerification, StateRecoveryFailed)
	case StateQuarantining:
		return oneOf(to, StateReconcilingGit, StateFinalVerification, StateRecoveryFailed)
	case StateReconcilingGit:
		return oneOf(to, StateApplyingRecovery, StateMitigatedWithDrift, StateRecoveryFailed)
	case StateApplyingRecovery:
		return oneOf(to, StateFinalVerification, StateMitigatedWithDrift, StateRecoveryFailed)
	case StateFinalVerification:
		return oneOf(to, StateRecovered, StateInvestigating, StateMitigatedWithDrift, StateRecoveryFailed, StateCircuitOpen)
	case StateMitigatedWithDrift:
		return oneOf(to, StateReconcilingGit, StateRecoveryFailed, StateEscalated)
	default:
		return false
	}
}

func oneOf(candidate State, allowed ...State) bool {
	for _, state := range allowed {
		if candidate == state {
			return true
		}
	}
	return false
}
