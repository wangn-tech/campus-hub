package model

type ActivityStatus uint8

// Activity status values follow the database design document (§16.2).
const (
	ActivityDraft ActivityStatus = iota
	ActivityPendingReview
	ActivityPublished
	ActivityOngoing
	ActivityFinished
	ActivityRejected
	ActivityCancelled
)

type RegistrationStatus uint8

// Registration status values follow the database design document (§16.3).
const (
	RegistrationPending RegistrationStatus = iota
	RegistrationApproved
	RegistrationRejected
	RegistrationCancelled
	RegistrationFailed
	RegistrationExpired
)

type TicketStatus uint8

// Ticket status values follow the database design document (§16.4).
const (
	TicketUnused TicketStatus = iota
	TicketUsed
	TicketExpired
	TicketVoided
)

type StudentVerificationStatus uint8

const (
	VerificationInitialized StudentVerificationStatus = iota
	VerificationOCRProcessing
	VerificationPendingConfirm
	VerificationManualReview
	VerificationApproved
	VerificationRejected
	VerificationExpired
	VerificationCancelled
	VerificationOCRFailed
)
