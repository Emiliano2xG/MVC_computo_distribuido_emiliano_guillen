package service

import (
	"strconv"
	"strings"
)

// Dias de la semana tal como aparecen en materias.dias ("Lunes y Miercoles").
// Se aceptan con y sin acento.
var nombresDia = map[string][]string{
	"lunes":     {"lunes"},
	"martes":    {"martes"},
	"miercoles": {"miercoles", "miércoles"},
	"jueves":    {"jueves"},
	"viernes":   {"viernes"},
	"sabado":    {"sabado", "sábado"},
}

func diasDe(texto string) map[string]bool {
	t := strings.ToLower(texto)
	out := map[string]bool{}
	for canon, variantes := range nombresDia {
		for _, v := range variantes {
			if strings.Contains(t, v) {
				out[canon] = true
			}
		}
	}
	return out
}

// minutos convierte "07:30" en minutos desde medianoche.
func minutos(hhmm string) (int, bool) {
	p := strings.Split(strings.TrimSpace(hhmm), ":")
	if len(p) != 2 {
		return 0, false
	}
	h, err1 := strconv.Atoi(p[0])
	m, err2 := strconv.Atoi(p[1])
	if err1 != nil || err2 != nil {
		return 0, false
	}
	return h*60 + m, true
}

// rango convierte "07:00-08:30" en (420, 510).
func rango(horario string) (int, int, bool) {
	p := strings.Split(horario, "-")
	if len(p) != 2 {
		return 0, 0, false
	}
	ini, ok1 := minutos(p[0])
	fin, ok2 := minutos(p[1])
	return ini, fin, ok1 && ok2 && fin > ini
}

// Empalman dice si dos materias coinciden en algun dia y sus horas se cruzan.
// Dos clases que solo se tocan en el borde (una termina a las 8:30 y la otra
// empieza a las 8:30) NO se empalman. Si un horario no se puede leer se
// considera que no hay empalme (no se bloquea por un dato mal capturado).
func Empalman(horarioA, diasA, horarioB, diasB string) bool {
	iniA, finA, okA := rango(horarioA)
	iniB, finB, okB := rango(horarioB)
	if !okA || !okB {
		return false
	}
	if !(iniA < finB && iniB < finA) {
		return false
	}
	b := diasDe(diasB)
	for d := range diasDe(diasA) {
		if b[d] {
			return true
		}
	}
	return false
}
