SART (Sistema Administrativo de Reporte y Soporte Tecnológico)

DOCUMENTACIÓN TÉCNICA Y FUNCIONAL
Fecha de Actualización: 06/02/2026

1. DESCRIPCIÓN GENERAL DEL SISTEMA
El Sistema Administrativo de Reporte y Soporte Tecnológico (SART) es una plataforma integral diseñada para la gestión, control, trazabilidad y mantenimiento del inventario tecnológico de la organización. Su propósito es centralizar la información de activos (hardware y redes) y gestionar su ciclo de vida completo, desde la asignación física hasta el soporte técnico.

2. ARQUITECTURA TÉCNICA (REQUERIMIENTOS DE RECONSTRUCCIÓN)
Para garantizar la operatividad bajo condiciones de recursos limitados y compatibilidad heredada, el sistema obedece a la siguiente arquitectura "Zero-Dependencies":

* Compatibilidad Objetivo: Windows 7 (32 bits) como mínimo.
* Entorno: Offline (Sin dependencia de internet).
* Backend: Lenguaje Go (Golang) compilado para arquitectura 386 (32 bits).
* Base de Datos: SQLite (Embebida, sin instalación de servidor externo).
* Frontend: HTML5, CSS3 y JS Vanilla (Sin frameworks pesados). Empaquetado dentro del binario usando `go:embed`.
* Entregable: Un único archivo ejecutable (.exe) portable.

3. MÓDULOS DEL SISTEMA

3.1. CONTROL DE ACCESO Y SEGURIDAD
* Autenticación: Inicio de sesión mediante credenciales locales (Usuario/Contraseña).
* Gestión de Sesiones: Persistencia segura y cierre de sesión (Logout).
* Roles de Usuario:
    -   Administrador: Control total (CRUD en todos los módulos, gestión de usuarios, tablas maestras y configuración).
    -   Visualizador/Consultor: Acceso de lectura, creación básica de tickets y consultas. Restricción en configuraciones críticas y eliminación de historial.

3.2. DASHBOARD (PANEL DE CONTROL)
Visualización inmediata de Indicadores Clave de Desempeño (KPIs) y estado del sistema:
* KPIs de Taller: Equipos actualmente en reparación, equipos reparados en el mes actual, histórico total de atendidos.
* Barra de Progreso Académico: Visualización gráfica del avance del período/semestre actual basado en las fechas de configuración.

3.3. MÓDULO DE INVENTARIO (GESTIÓN DE ACTIVOS)
Permite el registro detallado y la trazabilidad de bienes nacionales y equipos internos.
* Funciones CRUD: Alta, Baja, Modificación y Consulta de activos.
* Datos del Activo:
    -   Identificación: Código de Bien Nacional (BN), Serial, Código Interno.
    -   Clasificación: Tipo (PC, Laptop, Impresora), Marca, Modelo.
    -   Especificaciones Técnicas: Procesador, RAM, Almacenamiento, Arquitectura, S.O.
* Búsqueda Avanzada: Motor de búsqueda en tiempo real por serial, código o características técnicas. Incluye paginación.
* Ubicación Jerárquica (Trazabilidad): Asignación en estructura escalonada: Edificio > Piso > Área > Habitación/Oficina.

3.4. MÓDULO DE TALLER (INCIDENCIAS Y SOPORTE)
Administra el flujo de trabajo de reparaciones y mantenimiento correctivo.
* Ticket de Entrada:
    -   Vinculación directa desde el inventario.
    -   Registro de fecha de ingreso y descripción de la falla (`details_in`).
    -   Validación lógica: Impide duplicar ingresos de un mismo equipo con ticket abierto.
* Estado del Ticket: Control de flujos "Pendiente", "Reparado", "No Reparado".
* Ticket de Salida (Cierre):
    -   Registro de la solución técnica aplicada (`details_out`).
    -   Fecha de salida y cambio de estatus final.
* Historial de Mantenimiento: Bitácora de todas las intervenciones realizadas a un activo específico (Hoja de Vida).

3.5. MÓDULO DE REPORTES E HISTORIAL
Generación de documentos oficiales y auditoría de movimientos.
* Histórico General: Bitácora completa de entradas y salidas.
* Filtros de Reporte: Por rango de fechas, estatus, ubicación física o tipo de equipo.
* Formato de Impresión:
    -   Salida en PDF / Vista previa.
    -   Encabezados institucionales (Logos UNEFA).
    -   Pie de página dinámico con firmas de responsables (Jefe de Área / Coordinador).

3.6. ADMINISTRACIÓN Y CONFIGURACIÓN (MAESTROS)
Módulo exclusivo para administradores para garantizar la integridad de los datos (Normalización).
* Catálogos Maestros: Gestión estandarizada de Marcas, Modelos, Procesadores y Sistemas Operativos para evitar redundancia de datos.
* Configuración Institucional: Edición de nombres y cargos para las firmas en reportes.
* Gestión de Ubicaciones: Creación dinámica de la estructura geográfica (Edificios, Pisos, Departamentos).
* Control de Períodos: Definición de fechas de inicio y fin del período académico (afecta reportes y dashboard). Incluye lógica "Sticky Period" para visualización administrativa post-cierre.

4. GLOSARIO TÉCNICO
* BN (Bien Nacional): Identificador único de activos públicos.
* CRUD: Acrónimo de las operaciones básicas de datos (Create, Read, Update, Delete).
* Zero-Dependencies: Filosofía de software que no requiere instalación de librerías o motores externos (ej. no requiere instalar MySQL o .NET).
* Ticket: Registro digital de la solicitud de servicio.
* Maestros: Tablas de base de datos con valores predefinidos para estandarizar la entrada de información.