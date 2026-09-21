package imprint

import "github.com/SteamedBread2333/imprint/internal/vault/model"

type (
	Status       = model.Status
	EvidenceKind = model.EvidenceKind
	Evidence     = model.Evidence
	Record       = model.Record
)

const (
	StatusActive     = model.StatusActive
	StatusDormant    = model.StatusDormant
	StatusSuperseded = model.StatusSuperseded

	EvidenceOriginal  = model.EvidenceOriginal
	EvidenceReinforce = model.EvidenceReinforce
	EvidenceSupersede = model.EvidenceSupersede

	DefaultConfidence       = model.DefaultConfidence
	MaxConfidence           = model.MaxConfidence
	MinRecallConfidence     = model.MinRecallConfidence
	DefaultDecayDays        = model.DefaultDecayDays
	DefaultDecayAmount      = model.DefaultDecayAmount
	DefaultDormantThresh    = model.DefaultDormantThresh
	DefaultInheritanceAlpha = model.DefaultInheritanceAlpha
	DefaultTopK             = model.DefaultTopK
	IDPrefix                = model.IDPrefix
)
