CREATE TABLE IF NOT EXISTS carreras (
    id SERIAL PRIMARY KEY,
    clave VARCHAR(10) NOT NULL UNIQUE,
    nombre VARCHAR(120) NOT NULL
);

CREATE TABLE IF NOT EXISTS alumnos (
    id SERIAL PRIMARY KEY,
    nombre VARCHAR(255) NOT NULL,
    matricula VARCHAR(50) NOT NULL UNIQUE,
    correo VARCHAR(120) NOT NULL UNIQUE,
    password VARCHAR(120) NOT NULL,
    carrera_id INTEGER NOT NULL REFERENCES carreras(id)
);

CREATE TABLE IF NOT EXISTS materias (
    id SERIAL PRIMARY KEY,
    clave VARCHAR(20) NOT NULL UNIQUE,
    nombre VARCHAR(255) NOT NULL,
    profesor VARCHAR(255) NOT NULL,
    cupo INTEGER NOT NULL DEFAULT 30,
    horario VARCHAR(50) NOT NULL,
    dias VARCHAR(80) NOT NULL,
    semestre INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS carrera_materias (
    carrera_id INTEGER NOT NULL REFERENCES carreras(id),
    materia_id INTEGER NOT NULL REFERENCES materias(id),
    PRIMARY KEY (carrera_id, materia_id)
);

CREATE TABLE IF NOT EXISTS inscripciones (
    id SERIAL PRIMARY KEY,
    alumno_id INTEGER NOT NULL REFERENCES alumnos(id),
    materia_id INTEGER NOT NULL REFERENCES materias(id),
    estado VARCHAR(20) NOT NULL DEFAULT 'inscrita',
    UNIQUE (alumno_id, materia_id)
);

INSERT INTO carreras (clave, nombre) VALUES
    ('IDC', 'Inteligencia de Datos y Ciberseguridad'),
    ('MEC', 'Mecatronica'),
    ('IND', 'Ingenieria Industrial'),
    ('MCM', 'Ingenieria Mecanica')
ON CONFLICT (clave) DO NOTHING;

INSERT INTO alumnos (nombre, matricula, correo, password, carrera_id) VALUES
    ('Emiliano Guillen', '0250947', '0250947@up.edu.mx', 'emilianoguillen90', (SELECT id FROM carreras WHERE clave = 'IDC')),
    ('Mariana Lopez', '0251001', '0251001@up.edu.mx', 'marianalopez90', (SELECT id FROM carreras WHERE clave = 'MEC')),
    ('Diego Ramirez', '0251002', '0251002@up.edu.mx', 'diegoramirez90', (SELECT id FROM carreras WHERE clave = 'IND')),
    ('Ana Torres', '0251003', '0251003@up.edu.mx', 'anatorres90', (SELECT id FROM carreras WHERE clave = 'MCM')),
    ('Luis Herrera', '0251004', '0251004@up.edu.mx', 'luisherrera90', (SELECT id FROM carreras WHERE clave = 'IDC'))
ON CONFLICT (matricula) DO NOTHING;

INSERT INTO materias (clave, nombre, profesor, cupo, horario, dias, semestre) VALUES
    ('MAT31051', 'Calculo Diferencial', 'Perez', 32, '07:00-08:30', 'Lunes y Miercoles', 1),
    ('MAT31072', 'Algebra', 'Garcia', 32, '08:00-09:30', 'Martes y Jueves', 1),
    ('ETTF91', 'Quimica', 'Rojas', 28, '10:00-11:30', 'Lunes y Miercoles', 1),
    ('MEC31001', 'Fisica', 'Vargas', 28, '12:00-13:30', 'Martes y Jueves', 1),
    ('HUM30001', 'Historia de la Cultura', 'Nunez', 30, '14:00-15:30', 'Viernes', 1),
    ('MAT31073', 'Calculo Integral', 'Perez', 30, '07:00-08:30', 'Martes y Jueves', 2),
    ('MAT31074', 'Algebra Lineal', 'Garcia', 30, '08:00-09:30', 'Lunes y Miercoles', 2),
    ('LID31001', 'Liderazgo y Comunicacion Efectiva', 'Rojas', 26, '16:00-17:30', 'Miercoles', 2),
    ('HUM30002', 'Persona y Sociedad', 'Aguilar', 28, '11:30-13:00', 'Viernes', 2),
    ('MAT31075', 'Calculo Vectorial', 'Perez', 28, '07:00-08:30', 'Lunes y Miercoles', 3),
    ('MAT31076', 'Ecuaciones Diferenciales', 'Mendoza', 26, '09:30-11:00', 'Martes y Jueves', 3),
    ('OPT31001', 'Optativa I', 'Castro', 24, '16:00-17:30', 'Jueves', 3),
    ('HUM30003', 'Etica', 'Aguilar', 28, '13:00-14:30', 'Viernes', 3),
    ('MAT31080', 'Probabilidad y Estadistica', 'Mendoza', 28, '08:00-09:30', 'Lunes y Miercoles', 4),
    ('OPT31002', 'Optativa II', 'Castro', 24, '16:00-17:30', 'Martes', 4),
    ('HUM30004', 'Antropologia Teologica I', 'Aguilar', 26, '11:30-13:00', 'Viernes', 4),
    ('OPT31003', 'Optativa III', 'Castro', 24, '18:00-19:30', 'Miercoles', 5),
    ('HUM30008', 'Antropologia Teologica II', 'Aguilar', 26, '13:00-14:30', 'Viernes', 5),
    ('OPT31004', 'Optativa IV', 'Castro', 24, '16:00-17:30', 'Lunes', 6),
    ('HUM30005', 'Filosofia Social', 'Nunez', 26, '10:00-11:30', 'Viernes', 6),
    ('OPT31005', 'Optativa V', 'Castro', 24, '18:00-19:30', 'Jueves', 7),
    ('HUM30007', 'Hombre y Mundo Contemporaneo', 'Nunez', 26, '12:00-13:30', 'Viernes', 7),
    ('OPT31006', 'Optativa VI', 'Castro', 24, '16:00-17:30', 'Martes', 8),
    ('HUM30010', 'Etica Profesional', 'Aguilar', 26, '13:00-14:30', 'Viernes', 8),
    ('COM31001', 'Analisis y Diseno de Algoritmos', 'Navarro', 24, '10:00-11:30', 'Martes y Jueves', 1),
    ('COM31002', 'Programacion Orientada a Objetos', 'Sanchez', 26, '10:00-11:30', 'Lunes y Miercoles', 2),
    ('COM31037', 'Computacion Avanzada', 'Robin', 22, '12:00-13:30', 'Martes y Jueves', 2),
    ('COM31040', 'Comercializacion del Producto Digital', 'Flores', 22, '14:00-15:30', 'Lunes', 2),
    ('COM31004', 'Programacion y Estructura de Datos', 'Sanchez', 26, '10:00-11:30', 'Lunes y Miercoles', 3),
    ('COM31056', 'Procesamiento de Imagenes', 'Reyes', 20, '12:00-13:30', 'Martes y Jueves', 3),
    ('COM31012', 'Introduccion a las Bases de Datos', 'Lopez', 24, '14:00-15:30', 'Miercoles y Viernes', 3),
    ('DOP31002', 'Investigacion de Operaciones', 'Sosa', 24, '10:00-11:30', 'Martes y Jueves', 4),
    ('COM31009', 'Sistemas Operativos', 'Martinez', 22, '12:00-13:30', 'Lunes y Miercoles', 4),
    ('COM31020', 'Inteligencia Artificial', 'Navarro', 22, '14:00-15:30', 'Martes y Jueves', 4),
    ('COM31013', 'Bases de Datos Avanzadas', 'Lopez', 20, '16:00-17:30', 'Lunes y Miercoles', 4),
    ('COM31110', 'Matematicas de la Computacion', 'Ibarra', 22, '08:00-09:30', 'Martes y Jueves', 5),
    ('COM31111', 'Desarrollo de Aplicaciones Web', 'Castro', 22, '10:00-11:30', 'Lunes y Miercoles', 5),
    ('COM32004', 'Ciberseguridad', 'Ortega', 20, '12:00-13:30', 'Martes y Jueves', 5),
    ('COM31173', 'Procesamiento de Lenguaje Natural', 'Navarro', 18, '14:00-15:30', 'Lunes y Miercoles', 5),
    ('COM31147', 'Telemetria', 'Martinez', 18, '16:00-17:30', 'Viernes', 5),
    ('COM31533', 'Teoria de Lenguajes y Programacion', 'Sanchez', 20, '08:00-09:30', 'Lunes y Miercoles', 6),
    ('COM31219', 'Arquitectura de Computadoras', 'Ortega', 22, '10:00-11:30', 'Martes y Jueves', 6),
    ('COM31534', 'Desarrollo de Apps para Dispositivos Inteligentes', 'Reyes', 18, '12:00-13:30', 'Lunes y Miercoles', 6),
    ('COM31540', 'Ciberinteligencia', 'Ortega', 18, '14:00-15:30', 'Martes y Jueves', 6),
    ('COM31532', 'Ciencia de Datos para Negocios', 'Lopez', 20, '16:00-17:30', 'Miercoles', 6),
    ('COM31450', 'Interfaces Hombre-Maquina', 'Reyes', 18, '08:00-09:30', 'Martes y Jueves', 7),
    ('COM31466', 'Computo Distribuido', 'Robin', 22, '10:00-11:30', 'Lunes y Miercoles', 7),
    ('COM31378', 'Criptografia y Seguridad en Redes', 'Ortega', 18, '12:00-13:30', 'Martes y Jueves', 7),
    ('COM31480', 'Aprendizaje de Maquina', 'Navarro', 20, '14:00-15:30', 'Lunes y Miercoles', 7),
    ('COM31490', 'Datos Masivos', 'Lopez', 18, '16:00-17:30', 'Viernes', 7),
    ('COM31800', 'Herramientas y Tecnologias de CRM y ERP', 'Castro', 20, '08:00-09:30', 'Lunes y Miercoles', 8),
    ('COM31810', 'Hacking Etico y Recuperacion ante Desastres', 'Ortega', 16, '10:00-11:30', 'Martes y Jueves', 8),
    ('COM31820', 'Computo Forense', 'Martinez', 16, '12:00-13:30', 'Lunes y Miercoles', 8),
    ('COM31118', 'Ingenieria de Software', 'Castro', 22, '14:00-15:30', 'Martes y Jueves', 8),
    ('COM31830', 'Innovacion y Emprendimiento', 'Robin', 22, '16:00-17:30', 'Miercoles', 8),
    ('MEC32010', 'Estatica', 'Vargas', 24, '08:00-09:30', 'Lunes y Miercoles', 3),
    ('MEC32020', 'Dinamica', 'Vargas', 24, '10:00-11:30', 'Martes y Jueves', 3),
    ('MEC32030', 'Circuitos Electricos', 'Martinez', 22, '12:00-13:30', 'Lunes y Miercoles', 3),
    ('MEC32040', 'Electronica Analogica', 'Ortega', 20, '14:00-15:30', 'Martes y Jueves', 4),
    ('MEC32050', 'Electronica Digital', 'Ortega', 20, '16:00-17:30', 'Lunes y Miercoles', 4),
    ('MEC32060', 'Control Automatico', 'Sanchez', 20, '08:00-09:30', 'Martes y Jueves', 5),
    ('MEC32070', 'Robotica', 'Navarro', 18, '10:00-11:30', 'Lunes y Miercoles', 6),
    ('MEC32080', 'PLC y Automatizacion', 'Martinez', 18, '12:00-13:30', 'Martes y Jueves', 6),
    ('MEC32090', 'Diseno de Maquinas', 'Cruz', 20, '14:00-15:30', 'Viernes', 5),
    ('MEC32100', 'Sensores y Actuadores', 'Vargas', 18, '16:00-17:30', 'Miercoles', 5),
    ('MEC32110', 'Microcontroladores', 'Ortega', 18, '08:00-09:30', 'Lunes y Miercoles', 6),
    ('MEC32120', 'Sistemas Embebidos', 'Martinez', 16, '10:00-11:30', 'Viernes', 7),
    ('MEC32130', 'CAD CAM', 'Flores', 18, '12:00-13:30', 'Jueves', 4),
    ('MEC32140', 'Mecanismos', 'Cruz', 20, '14:00-15:30', 'Lunes', 5),
    ('MEC32150', 'Instrumentacion Industrial', 'Sosa', 18, '16:00-17:30', 'Martes', 7),
    ('IND32010', 'Ingenieria de Metodos', 'Sosa', 24, '08:00-09:30', 'Lunes y Miercoles', 3),
    ('IND32020', 'Estudio del Trabajo', 'Rojas', 24, '10:00-11:30', 'Martes y Jueves', 3),
    ('IND32030', 'Planeacion y Control de la Produccion', 'Castro', 22, '12:00-13:30', 'Lunes y Miercoles', 4),
    ('IND32040', 'Logistica', 'Delgado', 22, '14:00-15:30', 'Martes y Jueves', 4),
    ('IND32050', 'Control de Calidad', 'Sosa', 22, '16:00-17:30', 'Viernes', 5),
    ('IND32060', 'Simulacion de Sistemas', 'Ibarra', 20, '08:00-09:30', 'Martes y Jueves', 5),
    ('IND32070', 'Ergonomia', 'Reyes', 20, '10:00-11:30', 'Lunes y Miercoles', 5),
    ('IND32080', 'Gestion de Operaciones', 'Rojas', 22, '12:00-13:30', 'Miercoles y Viernes', 6),
    ('IND32090', 'Ingenieria de Costos', 'Sosa', 22, '14:00-15:30', 'Jueves', 6),
    ('IND32100', 'Inventarios', 'Delgado', 20, '16:00-17:30', 'Lunes', 6),
    ('IND32110', 'Cadena de Suministro', 'Rojas', 20, '08:00-09:30', 'Martes y Jueves', 7),
    ('IND32120', 'Seguridad Industrial', 'Cruz', 22, '10:00-11:30', 'Viernes', 4),
    ('IND32130', 'Diseno de Plantas', 'Castro', 18, '12:00-13:30', 'Lunes y Miercoles', 7),
    ('IND32140', 'Seis Sigma', 'Sosa', 18, '14:00-15:30', 'Martes', 7),
    ('IND32150', 'Manufactura Esbelta', 'Rojas', 20, '16:00-17:30', 'Miercoles', 8),
    ('MCM32010', 'Dibujo Mecanico', 'Flores', 24, '08:00-09:30', 'Lunes y Miercoles', 2),
    ('MCM32020', 'Ciencia de los Materiales', 'Vargas', 24, '10:00-11:30', 'Martes y Jueves', 3),
    ('MCM32030', 'Termodinamica', 'Perez', 22, '12:00-13:30', 'Lunes y Miercoles', 3),
    ('MCM32040', 'Mecanica de Fluidos', 'Vargas', 22, '14:00-15:30', 'Martes y Jueves', 4),
    ('MCM32050', 'Transferencia de Calor', 'Mendoza', 20, '16:00-17:30', 'Viernes', 4),
    ('MCM32060', 'Diseno Mecanico I', 'Cruz', 20, '08:00-09:30', 'Lunes y Miercoles', 5),
    ('MCM32070', 'Diseno Mecanico II', 'Cruz', 20, '10:00-11:30', 'Martes y Jueves', 6),
    ('MCM32080', 'Maquinas Termicas', 'Perez', 18, '12:00-13:30', 'Lunes y Miercoles', 6),
    ('MCM32090', 'Vibraciones Mecanicas', 'Vargas', 18, '14:00-15:30', 'Miercoles y Viernes', 6),
    ('MCM32100', 'Resistencia de Materiales', 'Garcia', 22, '16:00-17:30', 'Jueves', 4),
    ('MCM32110', 'Procesos de Manufactura', 'Sosa', 20, '08:00-09:30', 'Viernes', 5),
    ('MCM32120', 'Elementos de Maquinas', 'Cruz', 20, '10:00-11:30', 'Lunes y Miercoles', 5),
    ('MCM32130', 'Motores de Combustion', 'Perez', 18, '12:00-13:30', 'Martes', 7),
    ('MCM32140', 'Sistemas Termicos', 'Mendoza', 18, '14:00-15:30', 'Jueves', 7),
    ('MCM32150', 'Dinamica de Maquinaria', 'Vargas', 18, '16:00-17:30', 'Miercoles', 7),
    ('TLL-010', 'Taller de Innovacion', 'Robin', 12, '19:30-21:00', 'Martes', 8),
    ('TLL-001', 'Taller de Cupo Lleno', 'Prueba', 1, '19:30-21:00', 'Lunes', 8)
ON CONFLICT (clave) DO NOTHING;

INSERT INTO carrera_materias (carrera_id, materia_id)
SELECT c.id, m.id
FROM carreras c
JOIN materias m ON m.clave IN (
    'MAT31051','MAT31072','ETTF91','MEC31001','HUM30001',
    'MAT31073','MAT31074','LID31001','HUM30002',
    'MAT31075','MAT31076','OPT31001','HUM30003',
    'MAT31080','OPT31002','HUM30004',
    'OPT31003','HUM30008','OPT31004','HUM30005',
    'OPT31005','HUM30007','OPT31006','HUM30010',
    'TLL-010','TLL-001'
)
ON CONFLICT DO NOTHING;

INSERT INTO carrera_materias (carrera_id, materia_id)
SELECT c.id, m.id
FROM carreras c
JOIN materias m ON m.clave IN (
    'COM31001','COM31002','COM31037','COM31040','COM31004','COM31056','COM31012',
    'DOP31002','COM31009','COM31020','COM31013','COM31110','COM31111','COM32004',
    'COM31173','COM31147','COM31533','COM31219','COM31534','COM31540','COM31532',
    'COM31450','COM31466','COM31378','COM31480','COM31490','COM31800','COM31810',
    'COM31820','COM31118','COM31830'
)
WHERE c.clave = 'IDC'
ON CONFLICT DO NOTHING;

INSERT INTO carrera_materias (carrera_id, materia_id)
SELECT c.id, m.id
FROM carreras c
JOIN materias m ON m.clave LIKE 'MEC32%'
WHERE c.clave = 'MEC'
ON CONFLICT DO NOTHING;

INSERT INTO carrera_materias (carrera_id, materia_id)
SELECT c.id, m.id
FROM carreras c
JOIN materias m ON m.clave LIKE 'IND32%' OR m.clave = 'DOP31002'
WHERE c.clave = 'IND'
ON CONFLICT DO NOTHING;

INSERT INTO carrera_materias (carrera_id, materia_id)
SELECT c.id, m.id
FROM carreras c
JOIN materias m ON m.clave LIKE 'MCM32%'
WHERE c.clave = 'MCM'
ON CONFLICT DO NOTHING;

INSERT INTO carrera_materias (carrera_id, materia_id)
SELECT c.id, m.id
FROM carreras c
JOIN materias m ON m.clave IN ('MCM32030','MCM32020')
WHERE c.clave = 'MEC'
ON CONFLICT DO NOTHING;

INSERT INTO inscripciones (alumno_id, materia_id, estado)
SELECT a.id, m.id, v.estado
FROM (VALUES
    ('0250947', 'MAT31051', 'cursada'),
    ('0250947', 'MAT31072', 'cursada'),
    ('0250947', 'ETTF91', 'cursada'),
    ('0250947', 'MEC31001', 'cursada'),
    ('0250947', 'HUM30001', 'cursada'),
    ('0250947', 'COM31001', 'cursada'),
    ('0250947', 'MAT31073', 'cursada'),
    ('0250947', 'MAT31074', 'cursada'),
    ('0250947', 'LID31001', 'cursada'),
    ('0250947', 'HUM30002', 'cursada'),
    ('0250947', 'COM31002', 'cursada'),
    ('0250947', 'COM31037', 'cursada'),
    ('0250947', 'COM31040', 'cursada'),
    ('0250947', 'MAT31075', 'cursada'),
    ('0250947', 'MAT31076', 'cursada'),
    ('0250947', 'OPT31001', 'cursada'),
    ('0250947', 'HUM30003', 'cursada'),
    ('0250947', 'COM31004', 'cursada'),
    ('0250947', 'COM31056', 'cursada'),
    ('0250947', 'COM31012', 'cursada'),
    ('0250947', 'MAT31080', 'cursada'),
    ('0250947', 'OPT31002', 'cursada'),
    ('0250947', 'HUM30004', 'cursada'),
    ('0250947', 'DOP31002', 'cursada'),
    ('0250947', 'COM31009', 'cursada'),
    ('0250947', 'COM31020', 'cursada'),
    ('0250947', 'COM31013', 'cursada'),
    ('0250947', 'OPT31003', 'cursada'),
    ('0250947', 'HUM30008', 'cursada'),
    ('0250947', 'COM31110', 'cursada'),
    ('0250947', 'COM31111', 'cursada'),
    ('0250947', 'COM32004', 'cursada'),
    ('0250947', 'COM31173', 'cursada'),
    ('0250947', 'COM31147', 'cursada'),
    ('0250947', 'OPT31004', 'cursada'),
    ('0250947', 'HUM30005', 'cursada'),
    ('0250947', 'COM31533', 'cursada'),
    ('0250947', 'COM31219', 'cursada'),
    ('0250947', 'COM31534', 'cursada'),
    ('0250947', 'COM31540', 'cursada'),
    ('0250947', 'COM31532', 'cursada'),
    ('0250947', 'COM31466', 'inscrita'),
    ('0250947', 'COM31378', 'inscrita'),
    ('0250947', 'COM31480', 'inscrita'),
    ('0250947', 'COM31490', 'inscrita'),
    ('0250947', 'HUM30007', 'inscrita'),
    ('0251001', 'MAT31051', 'cursada'),
    ('0251001', 'MAT31072', 'cursada'),
    ('0251001', 'ETTF91', 'cursada'),
    ('0251001', 'MEC31001', 'cursada'),
    ('0251001', 'HUM30001', 'cursada'),
    ('0251001', 'MAT31073', 'cursada'),
    ('0251001', 'MAT31074', 'cursada'),
    ('0251001', 'LID31001', 'cursada'),
    ('0251001', 'HUM30002', 'cursada'),
    ('0251001', 'MAT31075', 'cursada'),
    ('0251001', 'MEC32010', 'cursada'),
    ('0251001', 'MEC32020', 'cursada'),
    ('0251001', 'MEC32030', 'cursada'),
    ('0251001', 'MEC32040', 'inscrita'),
    ('0251001', 'MEC32050', 'inscrita'),
    ('0251001', 'MEC32130', 'inscrita'),
    ('0251001', 'MAT31080', 'inscrita'),
    ('0251002', 'MAT31051', 'cursada'),
    ('0251002', 'MAT31072', 'cursada'),
    ('0251002', 'ETTF91', 'cursada'),
    ('0251002', 'MEC31001', 'cursada'),
    ('0251002', 'HUM30001', 'cursada'),
    ('0251002', 'IND32010', 'cursada'),
    ('0251002', 'IND32020', 'cursada'),
    ('0251002', 'IND32030', 'cursada'),
    ('0251002', 'IND32040', 'cursada'),
    ('0251002', 'IND32050', 'inscrita'),
    ('0251002', 'IND32060', 'inscrita'),
    ('0251002', 'IND32070', 'inscrita'),
    ('0251002', 'HUM30008', 'inscrita'),
    ('0251003', 'MAT31051', 'cursada'),
    ('0251003', 'MAT31072', 'cursada'),
    ('0251003', 'ETTF91', 'cursada'),
    ('0251003', 'MEC31001', 'cursada'),
    ('0251003', 'HUM30001', 'cursada'),
    ('0251003', 'MCM32010', 'cursada'),
    ('0251003', 'MCM32020', 'inscrita'),
    ('0251003', 'MCM32030', 'inscrita'),
    ('0251003', 'MAT31075', 'inscrita'),
    ('0251003', 'HUM30003', 'inscrita'),
    ('0251004', 'MAT31051', 'cursada'),
    ('0251004', 'MAT31072', 'cursada'),
    ('0251004', 'ETTF91', 'cursada'),
    ('0251004', 'MEC31001', 'cursada'),
    ('0251004', 'HUM30001', 'cursada')
) AS v(matricula, clave, estado)
JOIN alumnos a ON a.matricula = v.matricula
JOIN materias m ON m.clave = v.clave
ON CONFLICT (alumno_id, materia_id) DO UPDATE SET estado = EXCLUDED.estado;
