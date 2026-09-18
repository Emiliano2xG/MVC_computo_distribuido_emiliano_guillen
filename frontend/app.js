const API = "http://localhost:8000";
const DIAS = ["Lunes", "Martes", "Miercoles", "Jueves", "Viernes"];
const DIAS_LABEL = ["Lunes", "Martes", "Miércoles", "Jueves", "Viernes"];
const HORAS = [7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19];
const CLAVE_ALUMNO = "alumno_id";

const pantallaLogin = document.getElementById("pantalla-login");
const app = document.getElementById("app");
const listaMaterias = document.getElementById("lista-materias");
const calendario = document.getElementById("calendario");
const mensaje = document.getElementById("mensaje");
const nombreAlumno = document.getElementById("nombre-alumno");
const carreraAlumno = document.getElementById("carrera-alumno");
const tituloCatalogo = document.getElementById("titulo-catalogo");
const errorLogin = document.getElementById("error-login");
const errorRegistro = document.getElementById("error-registro");
const formLogin = document.getElementById("form-login");
const formNuevo = document.getElementById("form-nuevo");
const selectCarrera = document.getElementById("nuevo-carrera");
const loginTabs = document.querySelectorAll("[data-tab]");
const resumenHorario = document.getElementById("resumen-horario");

let alumno = null;

function mostrarMensaje(texto) {
    mensaje.textContent = texto;
}

async function pedir(ruta, opciones) {
    const resp = await fetch(API + ruta, opciones);
    if (!resp.ok) {
        const texto = await resp.text();
        throw new Error(texto || "error en la peticion");
    }
    if (resp.status === 204) {
        return null;
    }
    return resp.json();
}

function diasDe(texto) {
    const out = [];
    DIAS.forEach(function (dia) {
        if (texto.indexOf(dia) !== -1) {
            out.push(dia);
        }
    });
    return out;
}

function horaInicio(horario) {
    const parte = horario.split("-")[0];
    return parseInt(parte.split(":")[0], 10);
}

function limpiarErrores() {
    errorLogin.textContent = "";
    errorRegistro.textContent = "";
}

function mostrarVista(nombre) {
    document.querySelectorAll("[data-vista]").forEach(function (vista) {
        vista.hidden = vista.getAttribute("data-vista") !== nombre;
    });
    document.querySelectorAll("[data-nav]").forEach(function (btn) {
        btn.classList.toggle("is-active", btn.getAttribute("data-nav") === nombre);
    });
    const hash = nombre === "materias" ? "#materias" : "#horario";
    if (location.hash !== hash) {
        history.replaceState(null, "", hash);
    }
}

function mostrarPestana(nombre) {
    const esLogin = nombre === "entrar";
    formLogin.hidden = !esLogin;
    formNuevo.hidden = esLogin;
    loginTabs.forEach(function (btn) {
        const activa = btn.getAttribute("data-tab") === nombre;
        btn.classList.toggle("is-active", activa);
        btn.setAttribute("aria-selected", activa ? "true" : "false");
    });
    limpiarErrores();
}

function mostrarLogin() {
    pantallaLogin.hidden = false;
    app.hidden = true;
    document.body.classList.add("en-login");
    alumno = null;
    localStorage.removeItem(CLAVE_ALUMNO);
    mostrarPestana("entrar");
    cargarCarreras();
}

function entrarComo(datos) {
    alumno = datos;
    localStorage.setItem(CLAVE_ALUMNO, String(datos.id));
    nombreAlumno.textContent = datos.nombre;
    carreraAlumno.textContent = datos.carrera;
    tituloCatalogo.textContent = "Plan de " + datos.carrera + ". El tronco común se comparte con otras ingenierías.";
    pantallaLogin.hidden = true;
    app.hidden = false;
    document.body.classList.remove("en-login");
    mostrarVista(location.hash === "#materias" ? "materias" : "horario");
    mostrarMensaje("");
    cargarTodo();
}

async function cargarCarreras() {
    try {
        const carreras = await pedir("/alumnos/carreras");
        selectCarrera.innerHTML = "<option value=''>Elige tu carrera</option>";
        carreras.forEach(function (c) {
            const op = document.createElement("option");
            op.value = c.id;
            op.textContent = c.nombre;
            selectCarrera.appendChild(op);
        });
    } catch (err) {
        selectCarrera.innerHTML = "<option value=''>No se pudieron cargar las carreras</option>";
    }
}

function actualesDe(inscritas) {
    return (inscritas || []).filter(function (fila) {
        return fila.estado !== "cursada";
    });
}

function pintarCalendario(inscritas) {
    inscritas = actualesDe(inscritas);
    const cuantas = inscritas.length;
    resumenHorario.textContent = cuantas === 1 ? "1 materia inscrita" : cuantas + " materias inscritas";
    calendario.innerHTML = "";

    if (cuantas === 0) {
        const vacio = document.createElement("div");
        vacio.className = "vacio";
        vacio.innerHTML = "<p>Todavía no armas tu semana.</p>";
        const ir = document.createElement("button");
        ir.type = "button";
        ir.setAttribute("data-nav", "materias");
        ir.textContent = "Agregar materias";
        vacio.appendChild(ir);
        calendario.appendChild(vacio);
        return;
    }

    let horaMin = 19;
    let horaMax = 7;
    inscritas.forEach(function (clase) {
        const hora = horaInicio(clase.horario);
        if (hora < horaMin) {
            horaMin = hora;
        }
        if (hora > horaMax) {
            horaMax = hora;
        }
    });

    calendario.appendChild(document.createElement("div"));
    DIAS_LABEL.forEach(function (dia) {
        const titulo = document.createElement("div");
        titulo.className = "dia-titulo";
        titulo.textContent = dia;
        calendario.appendChild(titulo);
    });

    HORAS.filter(function (hora) {
        return hora >= horaMin && hora <= horaMax;
    }).forEach(function (hora) {
        const etiqueta = document.createElement("div");
        etiqueta.className = "celda-hora";
        etiqueta.textContent = hora + ":00";
        calendario.appendChild(etiqueta);

        DIAS.forEach(function (dia) {
            const slot = document.createElement("div");
            slot.className = "slot";

            inscritas.forEach(function (clase) {
                const vaEsteDia = diasDe(clase.dias).indexOf(dia) !== -1;
                if (vaEsteDia && horaInicio(clase.horario) === hora) {
                    const caja = document.createElement("div");
                    caja.className = "clase";
                    caja.innerHTML = "<strong>" + clase.clave + "</strong>" + clase.materia +
                        "<br>" + clase.horario;
                    const boton = document.createElement("button");
                    boton.type = "button";
                    boton.textContent = "Baja";
                    boton.onclick = function (ev) {
                        ev.stopPropagation();
                        darDeBaja(clase.id);
                    };
                    caja.appendChild(boton);
                    slot.appendChild(caja);
                }
            });

            calendario.appendChild(slot);
        });
    });
}

function pintarFilaMateria(materia, idsInscritos, idsCursadas) {
    const fila = document.createElement("div");
    fila.className = "fila";
    const lugares = materia.cupo - (materia.inscritos || 0);
    fila.innerHTML =
        "<div class='datos'><strong>" + materia.clave + " · " + materia.nombre + "</strong>" +
        "<span>" + materia.profesor + " · " + materia.dias + " · " + materia.horario +
        " · cupo " + lugares + "/" + materia.cupo + "</span></div>";

    if (idsCursadas[materia.id]) {
        const ok = document.createElement("button");
        ok.type = "button";
        ok.className = "lleno cursada";
        ok.textContent = "Cursada";
        fila.appendChild(ok);
    } else if (idsInscritos[materia.id]) {
        const boton = document.createElement("button");
        boton.type = "button";
        boton.className = "baja";
        boton.textContent = "Baja";
        boton.onclick = function () {
            darDeBaja(idsInscritos[materia.id]);
        };
        fila.appendChild(boton);
    } else if (lugares <= 0) {
        const lleno = document.createElement("button");
        lleno.className = "lleno";
        lleno.textContent = "Sin cupo";
        fila.appendChild(lleno);
    } else {
        const boton = document.createElement("button");
        boton.className = "alta";
        boton.textContent = "Alta";
        boton.onclick = function () {
            darDeAlta(materia.id);
        };
        fila.appendChild(boton);
    }
    return fila;
}

function pintarMaterias(materias, inscritas) {
    const idsInscritos = {};
    const idsCursadas = {};
    inscritas.forEach(function (fila) {
        if (fila.estado === "cursada") {
            idsCursadas[fila.materia_id] = true;
        } else {
            idsInscritos[fila.materia_id] = fila.id;
        }
    });

    listaMaterias.innerHTML = "";
    let semestreActual = -1;
    materias.forEach(function (materia) {
        if (materia.semestre !== semestreActual) {
            semestreActual = materia.semestre;
            const titulo = document.createElement("h3");
            titulo.className = "semestre";
            titulo.textContent = "Semestre " + semestreActual;
            listaMaterias.appendChild(titulo);
        }
        listaMaterias.appendChild(pintarFilaMateria(materia, idsInscritos, idsCursadas));
    });
}

async function cargarTodo() {
    if (!alumno) {
        return;
    }
    try {
        const materias = await pedir("/materias?carrera_id=" + alumno.carrera_id);
        const inscritas = await pedir("/inscripciones?alumno_id=" + alumno.id);
        pintarCalendario(inscritas);
        pintarMaterias(materias, inscritas);
    } catch (err) {
        mostrarMensaje(err.message);
    }
}

async function darDeAlta(materiaId) {
    try {
        await pedir("/inscripciones", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ alumno_id: alumno.id, materia_id: materiaId })
        });
        mostrarMensaje("Materia dada de alta");
        mostrarVista("horario");
        cargarTodo();
    } catch (err) {
        mostrarMensaje(err.message);
    }
}

async function darDeBaja(inscripcionId) {
    try {
        await pedir("/inscripciones/" + inscripcionId, { method: "DELETE" });
        mostrarMensaje("Materia dada de baja");
        cargarTodo();
    } catch (err) {
        mostrarMensaje(err.message);
    }
}

app.addEventListener("click", function (ev) {
    const destino = ev.target.closest("[data-nav]");
    if (destino) {
        mostrarVista(destino.getAttribute("data-nav"));
    }
});

loginTabs.forEach(function (btn) {
    btn.onclick = function () {
        mostrarPestana(btn.getAttribute("data-tab"));
    };
});

formLogin.onsubmit = async function (ev) {
    ev.preventDefault();
    limpiarErrores();
    try {
        const datos = await pedir("/alumnos/login", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                correo: document.getElementById("login-correo").value.trim(),
                password: document.getElementById("login-password").value
            })
        });
        entrarComo(datos);
    } catch (err) {
        errorLogin.textContent = err.message;
    }
};

formNuevo.onsubmit = async function (ev) {
    ev.preventDefault();
    limpiarErrores();
    try {
        const datos = await pedir("/alumnos", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                nombre: document.getElementById("nuevo-nombre").value.trim(),
                correo: document.getElementById("nuevo-correo").value.trim(),
                password: document.getElementById("nuevo-password").value,
                carrera_id: parseInt(selectCarrera.value, 10)
            })
        });
        entrarComo(datos);
    } catch (err) {
        errorRegistro.textContent = err.message;
    }
};

document.getElementById("btn-cambiar").onclick = function () {
    mostrarLogin();
};

async function iniciar() {
    const guardado = localStorage.getItem(CLAVE_ALUMNO);
    if (!guardado) {
        mostrarLogin();
        return;
    }
    try {
        const datos = await pedir("/alumnos/" + guardado);
        entrarComo(datos);
    } catch (err) {
        mostrarLogin();
    }
}

iniciar();
