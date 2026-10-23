package service

import "testing"

func TestEmpalman(t *testing.T) {
	casos := []struct {
		nombre         string
		hA, dA, hB, dB string
		quiero         bool
	}{
		{"mismo dia y misma hora", "07:00-08:30", "Lunes y Miercoles", "07:00-08:30", "Lunes", true},
		{"se cruzan a la mitad", "07:00-08:30", "Lunes", "08:00-09:30", "Lunes", true},
		{"una contiene a la otra", "07:00-10:00", "Martes", "08:00-09:00", "Martes", true},
		{"solo se tocan en el borde", "07:00-08:30", "Lunes", "08:30-10:00", "Lunes", false},
		{"misma hora, dias distintos", "07:00-08:30", "Lunes y Miercoles", "07:00-08:30", "Martes y Jueves", false},
		{"mismo dia, horas distintas", "07:00-08:30", "Lunes", "16:00-17:30", "Lunes", false},
		{"miercoles con acento", "07:00-08:30", "Miércoles", "07:00-08:30", "Miercoles", true},
		{"horario ilegible no bloquea", "sin horario", "Lunes", "07:00-08:30", "Lunes", false},
		{"dias vacios", "07:00-08:30", "", "07:00-08:30", "Lunes", false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := Empalman(c.hA, c.dA, c.hB, c.dB); got != c.quiero {
				t.Errorf("Empalman(%q,%q,%q,%q) = %v, queria %v", c.hA, c.dA, c.hB, c.dB, got, c.quiero)
			}
			// es simetrico
			if got := Empalman(c.hB, c.dB, c.hA, c.dA); got != c.quiero {
				t.Errorf("no es simetrico para %s", c.nombre)
			}
		})
	}
}
