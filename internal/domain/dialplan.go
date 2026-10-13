package domain

type IdleAction int

const (
	IdleNone IdleAction = iota
	IdleVoicemail
	IdleError
)

type Dialplan struct {
	VoicemailCode string
}

func NewDialplan(voicemailCode string) Dialplan {
	if voicemailCode == "" {
		voicemailCode = "*1"
	}
	return Dialplan{VoicemailCode: voicemailCode}
}

func (d Dialplan) MatchIdle(digits string) IdleAction {
	if digits == "" {
		return IdleNone
	}
	if digits == d.VoicemailCode {
		return IdleVoicemail
	}
	return IdleError
}
