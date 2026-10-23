"use strict";

const API = "http://localhost:8000";
const CLAVE_ALUMNO = "alumno_id";

const DIAS = ["Lunes", "Martes", "Miercoles", "Jueves", "Viernes"];
const DIAS_LABEL = ["Lunes", "Martes", "Miércoles", "Jueves", "Viernes"];
const DIAS_CORTO = ["Lun", "Mar", "Mié", "Jue", "Vie"];
const DIAS_ICS = ["MO", "TU", "WE", "TH", "FR"];

// Un color por materia (fondo suave + acento del borde).
const PALETA = [
    { fondo: "#f6e4d4", acento: "#c96b2b" },
    { fondo: "#e3ebdc", acento: "#5f8a5a" },
    { fondo: "#e1e8f1", acento: "#47689a" },
    { fondo: "#f1e6c9", acento: "#a67c1f" },
    { fondo: "#ecdfe9", acento: "#8a4f7d" },
    { fondo: "#dcebea", acento: "#3f8583" },
    { fondo: "#f0ddd8", acento: "#a8473a" }
];

const POLL_AVISOS_MS = 6000;
const POLL_DATOS_MS = 20000;

// ---------------------------------------------------------------------------
// Elementos
// ---------------------------------------------------------------------------
const $ = (id) => document.getElementById(id);

const pantallaLogin = $("pantalla-login");
const app = $("app");
const errorLogin = $("error-login");
const errorRegistro = $("error-registro");
const formLogin = $("form-login");
const formNuevo = $("form-nuevo");
const selectCarrera = $("nuevo-carrera");
const loginTabs = document.querySelectorAll("[data-tab]");

// ---------------------------------------------------------------------------
// Estado
// ---------------------------------------------------------------------------
let alumno = null;
let materias = [];
let inscritas = []; // inscripciones del alumno (estado inscrita o cursada)
let espera = []; // filas de lista de espera activas del alumno
let conteoEspera = {}; // materia_id -> cuantos esperan
let avisos = [];
let avisosVistos = new Set();
let avisosSinLeer = 0;
let primeraCargaAvisos = true;
let timerAvisos = null;
let timerDatos = null;
let filtro = { texto: "", semestre: "todos", estado: "todas" };
let semestresCerrados = new Set();
let vistaActual = "horario";

// ---------------------------------------------------------------------------
// Utilidades
// ---------------------------------------------------------------------------
async function pedir(ruta, opciones) {
    const resp = await fetch(API + ruta, opciones);
    if (!resp.ok) {
        let texto = (await resp.text()).trim();
        if (resp.status === 429) {
            texto = "Demasiadas peticiones seguidas, espera un momento.";
        }
        const err = new Error(texto || "Error en la petición (" + resp.status + ")");
        err.status = resp.status;
        throw err;
    }
    if (resp.status === 204) {
        return null;
    }
    return resp.json();
}

function enviar(metodo, cuerpo) {
    return {
        method: metodo,
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(cuerpo)
    };
}

function sinAcentos(texto) {
    return (texto || "").normalize("NFD").replace(/[\u0300-\u036f]/g, "");
}

function esc(texto) {
    const d = document.createElement("div");
    d.textContent = texto == null ? "" : String(texto);
    return d.innerHTML;
}

function diasDe(texto) {
    const t = sinAcentos(texto).toLowerCase();
    const out = [];
    DIAS.forEach(function (dia, i) {
        if (t.indexOf(dia.toLowerCase()) !== -1) {
            out.push(i);
        }
    });
    return out;
}

// "07:00-08:30" -> { ini: 420, fin: 510 } (minutos desde medianoche)
function rangoDe(horario) {
    const m = /(\d{1,2}):(\d{2})\s*-\s*(\d{1,2}):(\d{2})/.exec(horario || "");
    if (!m) {
        return null;
    }
    return { ini: +m[1] * 60 + +m[2], fin: +m[3] * 60 + +m[4] };
}

function hhmm(min) {
    return String(Math.floor(min / 60)).padStart(2, "0") + ":" + String(min % 60).padStart(2, "0");
}

// Misma regla que el servidor: comparten dia y los horarios se cruzan.
function empalman(a, b) {
    const ra = rangoDe(a.horario);
    const rb = rangoDe(b.horario);
    if (!ra || !rb) {
        return false;
    }
    if (!(ra.ini < rb.fin && rb.ini < ra.fin)) {
        return false;
    }
    const db = diasDe(b.dias);
    return diasDe(a.dias).some((d) => db.indexOf(d) !== -1);
}

function colorDe(materiaId) {
    return PALETA[Math.abs(materiaId) % PALETA.length];
}

function actuales() {
    return inscritas.filter((i) => i.estado !== "cursada");
}

function hace(fecha) {
    const seg = Math.max(0, Math.round((Date.now() - new Date(fecha).getTime()) / 1000));
    if (seg < 60) return "hace un momento";
    if (seg < 3600) return "hace " + Math.floor(seg / 60) + " min";
    if (seg < 86400) return "hace " + Math.floor(seg / 3600) + " h";
    return "hace " + Math.floor(seg / 86400) + " d";
}

// ---------------------------------------------------------------------------
// Avisos emergentes (toasts)
// ---------------------------------------------------------------------------
function toast(texto, tipo, titulo) {
    const caja = document.createElement("div");
    caja.className = "toast " + (tipo || "ok");
    const cuerpo = document.createElement("div");
    if (titulo) {
        const t = document.createElement("span");
        t.className = "toast-titulo";
        t.textContent = titulo;
        cuerpo.appendChild(t);
    }
    cuerpo.appendChild(document.createTextNode(texto));
    const cerrar = document.createElement("button");
    cerrar.type = "button";
    cerrar.setAttribute("aria-label", "Cerrar");
    cerrar.textContent = "×";
    cerrar.onclick = () => caja.remove();
    caja.appendChild(cuerpo);
    caja.appendChild(cerrar);
    $("toasts").appendChild(caja);
    setTimeout(() => caja.remove(), tipo === "error" ? 8000 : 5000);
}

// ---------------------------------------------------------------------------
// Navegacion
// ---------------------------------------------------------------------------
function mostrarVista(nombre) {
    vistaActual = nombre;
    document.querySelectorAll("[data-vista]").forEach(function (v) {
        v.hidden = v.getAttribute("data-vista") !== nombre;
    });
    document.querySelectorAll(".nav-btn[data-nav]").forEach(function (btn) {
        btn.classList.toggle("is-active", btn.getAttribute("data-nav") === nombre);
    });
    if (location.hash !== "#" + nombre) {
        history.replaceState(null, "", "#" + nombre);
    }
    window.scrollTo({ top: 0 });
}

function vistaInicial() {
    const h = location.hash.replace("#", "");
    return ["horario", "materias", "espera"].indexOf(h) !== -1 ? h : "horario";
}

// ---------------------------------------------------------------------------
// Login / registro
// ---------------------------------------------------------------------------
function limpiarErrores() {
    errorLogin.textContent = "";
    errorRegistro.textContent = "";
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

function detenerSondeos() {
    clearInterval(timerAvisos);
    clearInterval(timerDatos);
    timerAvisos = null;
    timerDatos = null;
}

function mostrarLogin() {
    detenerSondeos();
    pantallaLogin.hidden = false;
    app.hidden = true;
    document.body.classList.add("en-login");
    alumno = null;
    materias = [];
    inscritas = [];
    espera = [];
    avisos = [];
    avisosVistos = new Set();
    avisosSinLeer = 0;
    primeraCargaAvisos = true;
    $("panel-avisos").hidden = true;
    localStorage.removeItem(CLAVE_ALUMNO);
    mostrarPestana("entrar");
    cargarCarreras();
}

function saludo() {
    const h = new Date().getHours();
    const momento = h < 12 ? "Buenos días" : h < 19 ? "Buenas tardes" : "Buenas noches";
    return momento + ", " + alumno.nombre.split(" ")[0];
}

function entrarComo(datos) {
    alumno = datos;
    localStorage.setItem(CLAVE_ALUMNO, String(datos.id));
    $("nombre-alumno").textContent = datos.nombre;
    $("carrera-alumno").textContent = datos.carrera;
    $("saludo").textContent = saludo();
    $("titulo-catalogo").textContent =
        "Plan de " + datos.carrera + ". Te avisamos si una materia se llena o si empalma con tu horario.";
    filtro = { texto: "", semestre: "todos", estado: "disponible" };
    $("buscar").value = "";
    pantallaLogin.hidden = true;
    app.hidden = false;
    document.body.classList.remove("en-login");
    const traiaVista = location.hash !== "" && location.hash !== "#";
    mostrarVista(vistaInicial());
    cargarTodo().then(function () {
        // Si no tiene nada inscrito, lo mas util es llevarlo al catalogo.
        if (actuales().length === 0 && vistaActual === "horario" && !traiaVista) {
            mostrarVista("materias");
        }
    });
    cargarAvisos();
    detenerSondeos();
    timerAvisos = setInterval(cargarAvisos, POLL_AVISOS_MS);
    timerDatos = setInterval(function () {
        if (!document.hidden) {
            cargarTodo();
        }
    }, POLL_DATOS_MS);
}

// ---------------------------------------------------------------------------
// Datos
// ---------------------------------------------------------------------------
async function cargarTodo() {
    if (!alumno) {
        return;
    }
    const id = alumno.id;
    const t0 = performance.now();
    try {
        const [m, ins, esp, conteo] = await Promise.all([
            pedir("/materias?carrera_id=" + alumno.carrera_id),
            pedir("/inscripciones?alumno_id=" + id),
            pedir("/espera?alumno_id=" + id).catch(() => []),
            pedir("/espera/conteo").catch(() => ({}))
        ]);
        if (!alumno || alumno.id !== id) {
            return;
        }
        materias = m || [];
        inscritas = ins || [];
        espera = esp || [];
        conteoEspera = conteo || {};
        pintarTodo();
        estadoSistema(true, Math.round(performance.now() - t0));
    } catch (err) {
        estadoSistema(false);
        toast(err.message, "error", "No se pudo actualizar");
    }
}

async function estadoSistema(ok, ms) {
    const pie = $("pie-estado");
    pie.classList.toggle("mal", !ok);
    if (!ok) {
        pie.textContent = "Sin conexión con el sistema";
        return;
    }
    let atendido = "";
    try {
        const r = await fetch(API + "/heartbeat");
        const inst = r.headers.get("X-Instance-Id") || r.headers.get("X-Upstream");
        if (inst) {
            atendido = " · atendido por " + inst;
        }
    } catch (e) {
        /* el pie es solo informativo */
    }
    pie.textContent = "Sistema distribuido en línea · " + ms + " ms" + atendido;
}

function pintarTodo() {
    pintarTarjetas();
    pintarCalendario();
    pintarFiltros();
    pintarMaterias();
    pintarEspera();
    const b = $("badge-espera");
    b.hidden = espera.length === 0;
    b.textContent = espera.length;
}

// ---------------------------------------------------------------------------
// Mi horario: tarjetas de resumen
// ---------------------------------------------------------------------------
function clasesSemana() {
    // Una entrada por cada materia inscrita y dia en que se imparte.
    const out = [];
    actuales().forEach(function (c) {
        const r = rangoDe(c.horario);
        if (!r) {
            return;
        }
        diasDe(c.dias).forEach((d) => out.push({ clase: c, dia: d, ini: r.ini, fin: r.fin }));
    });
    return out;
}

function proximaClase(lista) {
    const ahora = new Date();
    const hoy = (ahora.getDay() + 6) % 7; // lunes = 0
    const min = ahora.getHours() * 60 + ahora.getMinutes();
    let mejor = null;
    lista.forEach(function (e) {
        let delta = (e.dia - hoy + 7) % 7;
        if (delta === 0 && e.fin <= min) {
            delta = 7;
        }
        const clave = delta * 1440 + e.ini;
        if (!mejor || clave < mejor.clave) {
            mejor = { clave: clave, delta: delta, e: e };
        }
    });
    return mejor;
}

function pintarTarjetas() {
    const cont = $("tarjetas");
    const lista = clasesSemana();
    const n = actuales().length;
    const minutos = lista.reduce((s, e) => s + (e.fin - e.ini), 0);
    const horas = Math.round((minutos / 60) * 10) / 10;

    const porDia = [0, 0, 0, 0, 0];
    lista.forEach((e) => (porDia[e.dia] += e.fin - e.ini));
    const libres = [];
    porDia.forEach((v, i) => {
        if (v === 0) libres.push(DIAS_LABEL[i]);
    });
    let notaHoras = "—";
    if (n) {
        notaHoras = libres.length
            ? "Día libre: " + libres.join(", ")
            : "Clases todos los días";
    }

    const prox = proximaClase(lista);
    let proxValor = "—";
    let proxNota = "Sin clases próximas";
    if (prox) {
        const cuando = prox.delta === 0 ? "Hoy" : prox.delta === 1 ? "Mañana" : DIAS_LABEL[prox.e.dia];
        proxValor = cuando + " · " + hhmm(prox.e.ini);
        proxNota = prox.e.clase.clave + " " + prox.e.clase.materia;
    }

    const cursadas = inscritas.filter((i) => i.estado === "cursada").length;
    const total = materias.length;
    const pct = total ? Math.round((cursadas / total) * 100) : 0;

    cont.innerHTML =
        "<div class='tarjeta'><span class='t-etiqueta'>Materias inscritas</span>" +
        "<span class='t-valor'>" + n + "</span>" +
        "<span class='t-nota'>" + (n ? "En este periodo" : "Aún no armas tu semana") + "</span></div>" +
        "<div class='tarjeta'><span class='t-etiqueta'>Horas de clase</span>" +
        "<span class='t-valor'>" + horas + " <small>por semana</small></span>" +
        "<span class='t-nota'>" + esc(notaHoras) + "</span></div>" +
        "<div class='tarjeta proxima'><span class='t-etiqueta'>Próxima clase</span>" +
        "<span class='t-valor'>" + esc(proxValor) + "</span>" +
        "<span class='t-nota'>" + esc(proxNota) + "</span></div>" +
        "<div class='tarjeta avance'><div class='anillo' style='--p:" + pct + "' data-texto='" + pct + "%'></div>" +
        "<div class='t-texto'><span class='t-etiqueta'>Avance</span>" +
        "<span class='t-nota'>" + cursadas + " de " + total + " materias cursadas</span></div></div>";
}

// ---------------------------------------------------------------------------
// Mi horario: calendario semanal (y agenda para pantallas chicas)
// ---------------------------------------------------------------------------
function pintarCalendario() {
    const cal = $("calendario");
    const agenda = $("agenda");
    const lista = clasesSemana();
    cal.innerHTML = "";
    agenda.innerHTML = "";
    const previo = document.querySelector("#vista-horario > .vacio");
    if (previo) {
        previo.remove();
    }

    if (lista.length === 0) {
        const vacio = document.createElement("div");
        vacio.className = "vacio";
        vacio.innerHTML =
            "<h3>Tu semana está libre</h3>" +
            "<p>Elige tus materias del catálogo y aquí verás tu horario armado, sin empalmes.</p>";
        const ir = document.createElement("button");
        ir.type = "button";
        ir.className = "btn btn-oscuro";
        ir.setAttribute("data-nav", "materias");
        ir.textContent = "Agregar materias";
        vacio.appendChild(ir);
        cal.parentElement.hidden = true;
        cal.parentElement.insertAdjacentElement("beforebegin", vacio);
        return;
    }
    cal.parentElement.hidden = false;

    // Rango visible: de la hora en punto anterior a la primera clase a la posterior a la ultima.
    let ini = Math.min.apply(null, lista.map((e) => e.ini));
    let fin = Math.max.apply(null, lista.map((e) => e.fin));
    ini = Math.max(6 * 60, Math.floor(ini / 60) * 60);
    fin = Math.min(23 * 60, Math.ceil(fin / 60) * 60);
    const filas = (fin - ini) / 30;
    const hoy = (new Date().getDay() + 6) % 7;

    // Encabezados
    cal.appendChild(document.createElement("div"));
    DIAS_LABEL.forEach(function (nombre, i) {
        const d = document.createElement("div");
        d.className = "cal-dia" + (i === hoy ? " hoy" : "");
        d.style.gridColumn = i + 2;
        const horasDia = lista.filter((e) => e.dia === i).length;
        d.innerHTML = nombre + "<small>" + (horasDia ? horasDia + (horasDia === 1 ? " clase" : " clases") : "libre") + "</small>";
        cal.appendChild(d);
    });

    // Horas y celdas de fondo
    for (let f = 0; f < filas; f++) {
        const minuto = ini + f * 30;
        const enPunto = minuto % 60 === 0;
        if (enPunto) {
            const h = document.createElement("div");
            h.className = "cal-hora";
            h.style.gridColumn = 1;
            h.style.gridRow = f + 2 + " / span 2";
            h.textContent = hhmm(minuto);
            cal.appendChild(h);
        }
        for (let d = 0; d < 5; d++) {
            const s = document.createElement("div");
            s.className = "cal-slot" + (enPunto ? " hora" : "") + (d === hoy ? " hoy" : "");
            s.style.gridColumn = d + 2;
            s.style.gridRow = f + 2;
            cal.appendChild(s);
        }
    }

    // Clases
    lista.forEach(function (e, idx) {
        const c = e.clase;
        const color = colorDe(c.materia_id);
        const desde = Math.max(ini, Math.floor(e.ini / 30) * 30);
        const hasta = Math.min(fin, Math.ceil(e.fin / 30) * 30);
        const caja = document.createElement("div");
        caja.className = "cal-clase";
        caja.style.setProperty("--fondo", color.fondo);
        caja.style.setProperty("--acento", color.acento);
        caja.style.gridColumn = e.dia + 2;
        caja.style.gridRow = (desde - ini) / 30 + 2 + " / span " + Math.max(1, (hasta - desde) / 30);

        // Si algun dato viejo ya empalma, se marca para que se note.
        const choca = lista.some((o, j) => j < idx && o.dia === e.dia && o.ini < e.fin && e.ini < o.fin);
        const choca2 = lista.some((o, j) => j > idx && o.dia === e.dia && o.ini < e.fin && e.ini < o.fin);
        if (choca || choca2) {
            caja.classList.add("choque");
            if (choca) caja.classList.add("derecha");
        }

        caja.innerHTML =
            "<strong>" + esc(c.clave) + "</strong>" +
            "<span class='c-nombre'>" + esc(c.materia) + "</span>" +
            "<span class='c-detalle'>" + hhmm(e.ini) + " - " + hhmm(e.fin) + "</span>" +
            (c.profesor ? "<span class='c-detalle'>" + esc(c.profesor) + "</span>" : "");
        const baja = document.createElement("button");
        baja.type = "button";
        baja.className = "c-baja";
        baja.textContent = "Dar de baja";
        baja.onclick = () => darDeBaja(c);
        caja.appendChild(baja);
        cal.appendChild(caja);
    });

    // Agenda por dia (pantallas chicas)
    DIAS_LABEL.forEach(function (nombre, d) {
        const delDia = lista.filter((e) => e.dia === d).sort((a, b) => a.ini - b.ini);
        if (!delDia.length) {
            return;
        }
        const bloque = document.createElement("div");
        bloque.className = "agenda-dia";
        const h = document.createElement("h3");
        h.textContent = nombre;
        if (d === hoy) h.classList.add("hoy");
        bloque.appendChild(h);
        delDia.forEach(function (e) {
            const color = colorDe(e.clase.materia_id);
            const it = document.createElement("div");
            it.className = "agenda-item";
            it.style.setProperty("--fondo", color.fondo);
            it.style.setProperty("--acento", color.acento);
            it.innerHTML =
                "<div><strong>" + hhmm(e.ini) + " - " + hhmm(e.fin) + "</strong>" +
                esc(e.clase.clave + " · " + e.clase.materia) +
                "<br><small>" + esc(e.clase.profesor || "") + "</small></div>";
            const b = document.createElement("button");
            b.type = "button";
            b.className = "btn btn-linea btn-peligro";
            b.textContent = "Baja";
            b.onclick = () => darDeBaja(e.clase);
            it.appendChild(b);
            bloque.appendChild(it);
        });
        agenda.appendChild(bloque);
    });
}

// ---------------------------------------------------------------------------
// Agregar materias: catalogo con busqueda y filtros
// ---------------------------------------------------------------------------
const CHIPS = [
    ["todas", "Todas"],
    ["disponible", "Disponibles"],
    ["inscrita", "Inscritas"],
    ["llena", "Sin cupo"],
    ["cursada", "Cursadas"]
];

// Estado de cada materia visto por este alumno.
function estadoDe(m) {
    const ins = inscritas.find((i) => i.materia_id === m.id);
    if (ins) {
        return { clave: ins.estado === "cursada" ? "cursada" : "inscrita", inscripcion: ins };
    }
    const fila = espera.find((e) => e.materia_id === m.id);
    if (fila) {
        return { clave: "espera", espera: fila };
    }
    if (m.cupo - (m.inscritos || 0) <= 0) {
        return { clave: "llena" };
    }
    const choque = actuales().find((a) => empalman(m, a));
    if (choque) {
        return { clave: "empalma", con: choque };
    }
    return { clave: "disponible" };
}

function coincideFiltro(m, est) {
    if (filtro.semestre !== "todos" && String(m.semestre) !== filtro.semestre) {
        return false;
    }
    if (filtro.texto) {
        const q = sinAcentos(filtro.texto).toLowerCase();
        const pajar = sinAcentos([m.clave, m.nombre, m.profesor].join(" ")).toLowerCase();
        if (pajar.indexOf(q) === -1) {
            return false;
        }
    }
    switch (filtro.estado) {
        case "disponible":
            return est.clave === "disponible";
        case "inscrita":
            return est.clave === "inscrita";
        case "llena":
            return est.clave === "llena" || est.clave === "espera";
        case "cursada":
            return est.clave === "cursada";
        default:
            return true;
    }
}

function pintarFiltros() {
    const sel = $("filtro-semestre");
    const semestres = Array.from(new Set(materias.map((m) => m.semestre))).sort((a, b) => a - b);
    const actual = filtro.semestre;
    sel.innerHTML = "<option value='todos'>Todos los semestres</option>" +
        semestres.map((s) => "<option value='" + s + "'>Semestre " + s + "</option>").join("");
    sel.value = actual;

    const cuentas = { todas: 0, disponible: 0, inscrita: 0, llena: 0, cursada: 0 };
    materias.forEach(function (m) {
        const est = estadoDe(m).clave;
        cuentas.todas++;
        if (est === "disponible") cuentas.disponible++;
        else if (est === "inscrita") cuentas.inscrita++;
        else if (est === "llena" || est === "espera") cuentas.llena++;
        else if (est === "cursada") cuentas.cursada++;
    });
    const cont = $("chips");
    cont.innerHTML = "";
    CHIPS.forEach(function (c) {
        const b = document.createElement("button");
        b.type = "button";
        b.className = "chip" + (filtro.estado === c[0] ? " is-active" : "");
        b.setAttribute("aria-pressed", filtro.estado === c[0] ? "true" : "false");
        b.innerHTML = esc(c[1]) + " <span>" + cuentas[c[0]] + "</span>";
        b.onclick = function () {
            filtro.estado = c[0];
            pintarFiltros();
            pintarMaterias();
        };
        cont.appendChild(b);
    });
}

function celdaCupo(m) {
    const libres = m.cupo - (m.inscritos || 0);
    const pct = m.cupo ? Math.min(100, Math.round(((m.inscritos || 0) / m.cupo) * 100)) : 100;
    const clase = libres <= 0 ? "lleno" : libres <= 3 ? "poco" : "";
    let texto;
    if (libres <= 0) {
        const n = conteoEspera[m.id] || 0;
        texto = n ? "Sin lugares · " + n + " en espera" : "Sin lugares";
    } else if (libres <= 3) {
        texto = "Quedan " + libres + (libres === 1 ? " lugar" : " lugares");
    } else {
        texto = libres + " de " + m.cupo + " lugares libres";
    }
    const d = document.createElement("div");
    d.className = "cupo " + clase;
    d.innerHTML = "<div class='cupo-barra'><i style='width:" + pct + "%'></i></div><small>" + esc(texto) + "</small>";
    return d;
}

function botonAccion(m, est) {
    const b = document.createElement("button");
    b.type = "button";
    b.className = "btn";
    switch (est.clave) {
        case "cursada":
            b.classList.add("btn-cursada");
            b.textContent = "Cursada";
            b.disabled = true;
            break;
        case "inscrita":
            b.classList.add("btn-linea", "btn-peligro");
            b.textContent = "Dar de baja";
            b.onclick = () => darDeBaja(est.inscripcion);
            break;
        case "espera":
            b.classList.add("btn-quieto");
            b.disabled = false;
            b.textContent = "En espera · lugar " + est.espera.posicion;
            b.onclick = () => mostrarVista("espera");
            break;
        case "llena":
            b.classList.add("btn-espera");
            b.textContent = "Anotarme en espera";
            b.onclick = () => anotarEspera(m, b);
            break;
        case "empalma":
            b.textContent = "Empalma";
            b.disabled = true;
            break;
        default:
            b.classList.add("btn-oscuro");
            b.textContent = "Dar de alta";
            b.onclick = () => darDeAlta(m, b);
    }
    return b;
}

function pintarMaterias() {
    const cont = $("lista-materias");
    cont.innerHTML = "";

    const visibles = materias
        .map((m) => ({ m: m, est: estadoDe(m) }))
        .filter((x) => coincideFiltro(x.m, x.est));
    $("conteo-materias").textContent =
        visibles.length === materias.length
            ? materias.length + " materias en tu plan"
            : "Mostrando " + visibles.length + " de " + materias.length + " materias";

    if (visibles.length === 0) {
        const v = document.createElement("div");
        v.className = "vacio";
        v.style.border = "0";
        v.innerHTML = "<h3>Sin resultados</h3><p>Prueba con otra búsqueda o quita los filtros.</p>";
        const b = document.createElement("button");
        b.type = "button";
        b.className = "btn btn-linea";
        b.textContent = "Limpiar filtros";
        b.onclick = function () {
            filtro = { texto: "", semestre: "todos", estado: "todas" };
            $("buscar").value = "";
            pintarFiltros();
            pintarMaterias();
        };
        v.appendChild(b);
        cont.appendChild(v);
        return;
    }

    const porSemestre = new Map();
    visibles.forEach(function (x) {
        if (!porSemestre.has(x.m.semestre)) porSemestre.set(x.m.semestre, []);
        porSemestre.get(x.m.semestre).push(x);
    });

    porSemestre.forEach(function (items, semestre) {
        const det = document.createElement("details");
        det.className = "semestre";
        det.open = !semestresCerrados.has(semestre);
        det.addEventListener("toggle", function () {
            if (det.open) semestresCerrados.delete(semestre);
            else semestresCerrados.add(semestre);
        });
        const cursadas = items.filter((x) => x.est.clave === "cursada").length;
        const disp = items.filter((x) => x.est.clave === "disponible").length;
        const sum = document.createElement("summary");
        sum.innerHTML = "Semestre " + semestre + "<small>" + cursadas + " cursadas · " + disp + " disponibles</small>";
        det.appendChild(sum);

        items.forEach(function (x) {
            const m = x.m;
            const fila = document.createElement("div");
            fila.className = "fila" + (x.est.clave === "cursada" ? " cursada" : "");
            const datos = document.createElement("div");
            datos.className = "datos";
            datos.innerHTML =
                "<strong>" + esc(m.clave) + " · " + esc(m.nombre) + "</strong>" +
                "<span>" + esc(m.profesor) + " · " + esc(m.dias) + " · " + esc(m.horario) + "</span>";
            if (x.est.clave === "empalma") {
                const c = x.est.con;
                const av = document.createElement("span");
                av.className = "aviso-fila";
                av.textContent = "Empalma con " + c.clave + " " + c.materia + " (" + c.dias + " " + c.horario + ")";
                datos.appendChild(av);
            }
            fila.appendChild(datos);
            fila.appendChild(x.est.clave === "cursada" ? document.createElement("div") : celdaCupo(m));
            const acc = document.createElement("div");
            acc.className = "accion";
            acc.appendChild(botonAccion(m, x.est));
            fila.appendChild(acc);
            det.appendChild(fila);
        });
        cont.appendChild(det);
    });
}

// ---------------------------------------------------------------------------
// Lista de espera
// ---------------------------------------------------------------------------
function pintarEspera() {
    const cont = $("lista-espera");
    cont.innerHTML = "";
    if (espera.length === 0) {
        const v = document.createElement("div");
        v.className = "vacio";
        v.style.gridColumn = "1 / -1";
        v.innerHTML =
            "<h3>No estás en ninguna fila</h3>" +
            "<p>Cuando una materia que quieres no tenga lugares, aparecerá aquí el botón <b>Anotarme en espera</b>.</p>";
        const b = document.createElement("button");
        b.type = "button";
        b.className = "btn btn-oscuro";
        b.setAttribute("data-nav", "materias");
        b.textContent = "Ver catálogo";
        v.appendChild(b);
        cont.appendChild(v);
        return;
    }
    espera.forEach(function (e) {
        const card = document.createElement("div");
        card.className = "espera-card";
        let puntos = "";
        for (let i = 1; i <= Math.min(e.en_fila, 12); i++) {
            puntos += "<i" + (i === e.posicion ? " class='tu'" : "") + "></i>";
        }
        const delante = e.posicion - 1;
        card.innerHTML =
            "<h3><small>" + esc(e.materia_clave) + "</small>" + esc(e.materia_nombre) + "</h3>" +
            "<div class='espera-lugar'>Tu lugar <b>" + e.posicion + "</b> de " + e.en_fila + "</div>" +
            "<div class='fila-puntos' aria-hidden='true'>" + puntos + "</div>" +
            "<p>" + (delante === 0
                ? "Eres el siguiente. En cuanto alguien se dé de baja te inscribimos automáticamente."
                : delante + (delante === 1 ? " persona va" : " personas van") + " antes que tú. Te inscribimos solos cuando te toque.") + "</p>";
        const b = document.createElement("button");
        b.type = "button";
        b.className = "btn btn-linea btn-peligro";
        b.textContent = "Salir de la fila";
        b.onclick = () => salirEspera(e);
        card.appendChild(b);
        cont.appendChild(card);
    });
}

// ---------------------------------------------------------------------------
// Acciones
// ---------------------------------------------------------------------------
async function darDeAlta(m, boton) {
    boton.disabled = true;
    try {
        await pedir("/inscripciones", enviar("POST", { alumno_id: alumno.id, materia_id: m.id }));
        toast(m.clave + " · " + m.nombre + " quedó en tu horario.", "ok", "Inscrita");
    } catch (err) {
        toast(err.message, "error", "No se pudo inscribir");
    }
    await cargarTodo();
}

async function darDeBaja(ins) {
    const nombre = ins.materia || ins.clave;
    if (!confirm("¿Dar de baja " + ins.clave + " · " + nombre + "?\nSu lugar se libera para otra persona.")) {
        return;
    }
    try {
        await pedir("/inscripciones/" + ins.id, { method: "DELETE" });
        toast(ins.clave + " · " + nombre + " ya no está en tu horario.", "ok", "Baja realizada");
    } catch (err) {
        toast(err.message, "error", "No se pudo dar de baja");
    }
    await cargarTodo();
}

async function anotarEspera(m, boton) {
    boton.disabled = true;
    try {
        const e = await pedir("/espera", enviar("POST", { alumno_id: alumno.id, materia_id: m.id }));
        toast("Eres el lugar " + e.posicion + " de " + e.en_fila + " en " + m.clave + ". Te inscribimos solos si se libera un lugar.", "aviso", "En lista de espera");
    } catch (err) {
        toast(err.message, "error", "No se pudo anotar");
    }
    await cargarTodo();
}

async function salirEspera(e) {
    if (!confirm("¿Salir de la fila de " + e.materia_clave + " · " + e.materia_nombre + "? Perderás tu lugar.")) {
        return;
    }
    try {
        await pedir("/espera/" + e.id, { method: "DELETE" });
        toast("Saliste de la fila de " + e.materia_clave + ".", "ok", "Listo");
    } catch (err) {
        toast(err.message, "error", "No se pudo salir");
    }
    await cargarTodo();
}

// ---------------------------------------------------------------------------
// Avisos (notificaciones que llegan por Kafka)
// ---------------------------------------------------------------------------
function claseAviso(tipo) {
    if (tipo === "inscripcion.cancelada" || tipo === "espera.descartada") return "baja";
    if (tipo.indexOf("espera.") === 0) return "espera";
    return "ok";
}

function pintarAvisos() {
    const b = $("badge-avisos");
    b.hidden = avisosSinLeer === 0;
    b.textContent = avisosSinLeer > 9 ? "9+" : avisosSinLeer;

    const ul = $("lista-avisos");
    ul.innerHTML = "";
    if (avisos.length === 0) {
        ul.innerHTML = "<li class='avisos-vacio'>Todavía no tienes avisos.<br>Aquí verás tus altas, bajas y cambios en la lista de espera.</li>";
        return;
    }
    avisos.slice(0, 20).forEach(function (a, i) {
        const li = document.createElement("li");
        li.className = "aviso " + claseAviso(a.tipo) + (i < avisosSinLeerVisibles ? " nuevo" : "");
        li.innerHTML = "<span class='punto'></span><div><p>" + esc(a.mensaje) + "</p><small>" + hace(a.recibida_en) + "</small></div>";
        ul.appendChild(li);
    });
}

let avisosSinLeerVisibles = 0;

async function cargarAvisos() {
    if (!alumno) {
        return;
    }
    const id = alumno.id;
    let lista;
    try {
        lista = await pedir("/notificaciones?limit=20&destinatario=" + encodeURIComponent(alumno.correo));
    } catch (err) {
        return;
    }
    if (!alumno || alumno.id !== id) {
        return;
    }
    lista = (lista || []).sort((a, b) => new Date(b.recibida_en) - new Date(a.recibida_en));
    const nuevos = lista.filter((a) => !avisosVistos.has(a.evento_id));
    lista.forEach((a) => avisosVistos.add(a.evento_id));
    avisos = lista;

    if (primeraCargaAvisos) {
        // Lo viejo no se anuncia: solo cuenta lo que llegue a partir de ahora.
        primeraCargaAvisos = false;
        avisosSinLeerVisibles = 0;
    } else if (nuevos.length) {
        avisosSinLeer += nuevos.length;
        avisosSinLeerVisibles = avisosSinLeer;
        nuevos.slice().reverse().forEach(function (a) {
            const tipo = claseAviso(a.tipo) === "baja" ? "error" : a.tipo.indexOf("espera.") === 0 ? "aviso" : "ok";
            const titulo = a.tipo === "espera.promovida" ? "¡Te dieron un lugar!" : "Aviso";
            toast(a.mensaje, tipo, titulo);
        });
        // Cambios hechos por el sistema (p. ej. una promocion desde la lista de espera).
        cargarTodo();
    }
    pintarAvisos();
}

function alternarAvisos(abrir) {
    const panel = $("panel-avisos");
    const mostrar = abrir === undefined ? panel.hidden : abrir;
    panel.hidden = !mostrar;
    $("btn-avisos").setAttribute("aria-expanded", mostrar ? "true" : "false");
    if (mostrar) {
        pintarAvisos();
        avisosSinLeer = 0;
        $("badge-avisos").hidden = true;
    } else {
        avisosSinLeerVisibles = 0;
    }
}

// ---------------------------------------------------------------------------
// Exportar a calendario (.ics)
// ---------------------------------------------------------------------------
function exportarICS() {
    const lista = clasesSemana();
    if (!lista.length) {
        toast("Primero inscribe alguna materia para exportar tu horario.", "aviso", "Horario vacío");
        return;
    }
    const pad = (n) => String(n).padStart(2, "0");
    const hoy = new Date();
    const lunes = new Date(hoy.getFullYear(), hoy.getMonth(), hoy.getDate() - ((hoy.getDay() + 6) % 7));
    const sello = hoy.toISOString().replace(/[-:]/g, "").replace(/\.\d+/, "");
    const lineas = [
        "BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Horario UP//ES", "CALSCALE:GREGORIAN",
        "X-WR-CALNAME:Horario UP", "X-WR-TIMEZONE:America/Mexico_City"
    ];
    lista.forEach(function (e, i) {
        const dia = new Date(lunes.getFullYear(), lunes.getMonth(), lunes.getDate() + e.dia);
        const f = dia.getFullYear() + pad(dia.getMonth() + 1) + pad(dia.getDate());
        const t = (min) => pad(Math.floor(min / 60)) + pad(min % 60) + "00";
        lineas.push(
            "BEGIN:VEVENT",
            "UID:horario-" + alumno.id + "-" + e.clase.id + "-" + e.dia + "-" + i + "@horarios-up",
            "DTSTAMP:" + sello,
            "DTSTART;TZID=America/Mexico_City:" + f + "T" + t(e.ini),
            "DTEND;TZID=America/Mexico_City:" + f + "T" + t(e.fin),
            "RRULE:FREQ=WEEKLY;COUNT=18;BYDAY=" + DIAS_ICS[e.dia],
            "SUMMARY:" + e.clase.clave + " " + e.clase.materia,
            "DESCRIPTION:" + (e.clase.profesor ? "Profesor: " + e.clase.profesor : "Clase"),
            "END:VEVENT"
        );
    });
    lineas.push("END:VCALENDAR");
    const blob = new Blob([lineas.join("\r\n")], { type: "text/calendar;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "horario-up.ics";
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
    toast("Ábrelo para añadir tus clases semanales a Google Calendar, Outlook o Apple Calendar.", "ok", "Archivo descargado");
}

// ---------------------------------------------------------------------------
// Eventos
// ---------------------------------------------------------------------------
app.addEventListener("click", function (ev) {
    const destino = ev.target.closest("[data-nav]");
    if (destino) {
        mostrarVista(destino.getAttribute("data-nav"));
    }
});

$("btn-avisos").onclick = (ev) => {
    ev.stopPropagation();
    alternarAvisos();
};
document.addEventListener("click", function (ev) {
    if (!$("panel-avisos").hidden && !ev.target.closest(".avisos-wrap")) {
        alternarAvisos(false);
    }
});
document.addEventListener("keydown", function (ev) {
    if (ev.key === "Escape") {
        alternarAvisos(false);
    }
});

$("btn-ics").onclick = exportarICS;
$("btn-imprimir").onclick = () => {
    mostrarVista("horario");
    window.print();
};

$("buscar").addEventListener("input", function (ev) {
    filtro.texto = ev.target.value.trim();
    pintarMaterias();
});
$("filtro-semestre").addEventListener("change", function (ev) {
    filtro.semestre = ev.target.value;
    pintarMaterias();
});

loginTabs.forEach(function (btn) {
    btn.onclick = () => mostrarPestana(btn.getAttribute("data-tab"));
});

document.querySelectorAll(".ver-pass").forEach(function (btn) {
    btn.onclick = function () {
        const campo = $(btn.getAttribute("data-ver"));
        const oculto = campo.type === "password";
        campo.type = oculto ? "text" : "password";
        btn.textContent = oculto ? "Ocultar" : "Ver";
        btn.setAttribute("aria-label", oculto ? "Ocultar contraseña" : "Mostrar contraseña");
    };
});

document.querySelectorAll(".demo").forEach(function (btn) {
    btn.onclick = function () {
        $("login-correo").value = btn.getAttribute("data-correo");
        $("login-password").value = btn.getAttribute("data-pass");
        mostrarPestana("entrar");
        formLogin.requestSubmit();
    };
});

formLogin.onsubmit = async function (ev) {
    ev.preventDefault();
    limpiarErrores();
    try {
        const datos = await pedir("/alumnos/login", enviar("POST", {
            correo: $("login-correo").value.trim(),
            password: $("login-password").value
        }));
        entrarComo(datos);
    } catch (err) {
        errorLogin.textContent = err.message;
    }
};

formNuevo.onsubmit = async function (ev) {
    ev.preventDefault();
    limpiarErrores();
    try {
        const datos = await pedir("/alumnos", enviar("POST", {
            nombre: $("nuevo-nombre").value.trim(),
            correo: $("nuevo-correo").value.trim(),
            password: $("nuevo-password").value,
            carrera_id: parseInt(selectCarrera.value, 10)
        }));
        entrarComo(datos);
    } catch (err) {
        errorRegistro.textContent = err.message;
    }
};

$("btn-cambiar").onclick = mostrarLogin;

window.addEventListener("hashchange", function () {
    if (alumno) {
        const h = vistaInicial();
        if (h !== vistaActual) mostrarVista(h);
    }
});

async function iniciar() {
    const guardado = localStorage.getItem(CLAVE_ALUMNO);
    if (!guardado) {
        mostrarLogin();
        return;
    }
    try {
        entrarComo(await pedir("/alumnos/" + guardado));
    } catch (err) {
        mostrarLogin();
    }
}

iniciar();
