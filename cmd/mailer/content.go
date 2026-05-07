package main

type emailLog struct {
	id        string
	emailType string
	requestID string
}

type recipient struct {
	email string
	name  string
}

func emailContent(emailType string) (subject, body string) {
	switch emailType {
	case "route_updated":
		return "Maršrutas atnaujintas", "Maršrutas, kuriame dalyvaujate, buvo atnaujintas vairuotojo."
	case "route_cancelled":
		return "Maršrutas atšauktas", "Maršrutas, kuriame dalyvaujate, buvo atšauktas."
	case "application_approved":
		return "Prašymas patvirtintas", "Jūsų prašymas prisijungti prie maršruto buvo patvirtintas."
	case "stop_change_approved":
		return "Stotelės keitimas patvirtintas", "Jūsų stotelės keitimo prašymas buvo patvirtintas."
	default:
		return "Pranešimas", "Turite naują pranešimą."
	}
}
