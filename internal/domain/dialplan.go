package domain

type IdleAction int

const (
	IdleNone IdleAction = iota
	IdleVoicemail
	IdleDictation
	IdleError
)

type Dialplan struct {
	VoicemailCode     string
	IdleDictationCode string
}

func NewDialplan(voicemailCode, idleDictationCode string) Dialplan {
	if voicemailCode == "" {
		voicemailCode = "*1"
	}
	if idleDictationCode == "" {
		idleDictationCode = "*2"
	}
	return Dialplan{VoicemailCode: voicemailCode, IdleDictationCode: idleDictationCode}
}

func (d Dialplan) MatchIdle(digits string) IdleAction {
	if digits == "" {
		return IdleNone
	}
	if digits == d.VoicemailCode {
		return IdleVoicemail
	}
	if digits == d.IdleDictationCode {
		return IdleDictation
	}
	return IdleError
}
