# GUÍA DE PRUEBAS - SISTEMA SART

## ⚠️ IMPORTANTE: VISUALIZAR LOGS DE DIAGNÓSTICO

Antes de hacer pruebas, abre la consola del navegador:
- **Chrome/Edge**: F12 → Tab "Console"
- **Firefox**: F12 → Tab "Console"
- Busca mensajes que comiencen con `[v0]` o `[DIAG]`

---

## PRUEBA 1: PERSISTENCIA DE SESIÓN ✅

### Paso 1: Iniciar sesión
1. Ingresa con usuario: `admin` / contraseña: `1234`
2. Verifica en la consola: `[v0] Login exitoso para usuario: admin`
3. En el header debes ver: **"OSWALDO GUEDEZ"** (en mayúsculas)

### Paso 2: Refrescar página
1. Presiona F5 o Ctrl+R
2. **NO debe pedirte login nuevamente**
3. Deberías permanecer conectado
4. Verifica consola: `[v0] Sesión restaurada para: OSWALDO GUEDEZ`

### Paso 3: Cerrar sesión
1. Haz click en "Cerrar Sesión"
2. Deberías volver a login
3. Verifica consola: `[v0] Cerrando sesión...`

---

## PRUEBA 2: UBICACIÓN JERÁRQUICA 🏢

### Paso 1: Abrir modal "Añadir al Taller"
1. Ve a sección "Taller"
2. Haz click en botón "Añadir"
3. En el modal, selecciona **"Edificio 01"**

### Paso 2: Verificar carga de pisos
- En la consola verás: `[DIAG] Buscando pisos para edificio: Edificio 01`
- El dropdown "Piso" debe popularse automáticamente
- Selecciona un piso (ej: "Piso 01")

### Paso 3: Verificar carga de áreas
- En la consola: `[DIAG] Buscando áreas para: Piso 01`
- El dropdown "Área" debe llenarse
- Selecciona un área (ej: "Área TIC")

### Paso 4: Verificar dispositivos
- En la consola: `[DIAG] Dispositivos encontrados en ubicación`
- El dropdown "Seleccionar Dispositivo" muestra:
  ```
  PC - Soporte Técnico - Dell - --- - CN-0N8176...
  ```
  (Formato: Tipo - Ubicación - Marca - Modelo - Serial)

---

## PRUEBA 3: VALIDACIÓN DE FECHA DE INGRESO 📅

### Paso 1: Intentar fecha futura
1. En modal "Añadir al Taller"
2. Selecciona edificio → piso → área → dispositivo
3. En "Fecha Ingreso", intenta seleccionar una fecha **FUTURA** (ej: mañana)
4. Deberías ver alerta: **"La fecha de ingreso no puede ser mayor a hoy"**
5. En consola: `[v0] Fecha futura rechazada: 2026-02-15`

### Paso 2: Fecha válida
1. Selecciona una fecha **HOY** o **PASADA**
2. El formulario debe aceptarla
3. Completa los datos y haz click "Guardar Datos"
4. En consola: `[v0] Guardando ticket - Fecha ingreso: 2026-02-01`

---

## PRUEBA 4: BARRA DE PROGRESO DE PERÍODO 📊

### Paso 1: Ver período actual
1. Ve a "Inicio"
2. Debes ver una tarjeta azul con "Período Académico"
3. Muestra el código del período (ej: "I-2026")
4. Muestra fechas de inicio y fin
5. Barra de progreso indica avance (0-100%)

### Paso 2: Editar período (solo para ADMIN)
1. Ve a "Configuración"
2. En la tabla "Período Académico Activo"
3. Debes ver solo el período ACTUAL
4. Los campos de fecha son editables
5. Haz cambios y haz click "Guardar"
6. En consola: `[DIAG] Actualizando período I-2026`

---

## PRUEBA 5: BOTONES CANCELAR ❌

### Paso 1: Modal de Ticket
1. Ve a "Taller" → Haz click "Añadir"
2. Se abre modal "Añadir al Taller"
3. Haz click en botón "Cancelar" (parte inferior)
4. Modal debe cerrarse
5. En consola: `[v0] Cerrando modal ticket`

### Paso 2: Modal de Finalizar
1. Ve a "Taller"
2. Si hay tickets pendientes, haz click "Finalizar" en uno
3. Se abre modal "Finalizar Servicio"
4. Haz click en botón "Cancelar"
5. Modal debe cerrarse
6. En consola: `[v0] Cerrando modal finish`

---

## PRUEBA 6: PAGINACIÓN EN INVENTARIO 📄

### Paso 1: Verificar paginación
1. Ve a "Inventario" (solo para ADMIN)
2. Debes ver tabla de dispositivos
3. Abajo verás indicador: "Pág X de Y (total)"
4. Ejemplo: "Pág 1 de 3 (28)"

### Paso 2: Botones de paginación
1. Si hay más de una página, verás botones "«" y "»"
2. Haz click en "»" (siguiente)
3. Deberías ver página 2
4. En consola: `[v0] Página siguiente. Página actual: 1`
5. El botón "«" (anterior) ahora está habilitado
6. El botón "»" se deshabilita si estás en última página

### Paso 3: Filtros y paginación
1. Selecciona un filtro (ej: Tipo = "PC")
2. Página debe resetear a 1
3. En consola: `[v0] Cargando dispositivos - Página: 1 Q: ""`

---

## PRUEBA 7: NOMBRE DE USUARIO EN HEADER 👤

### Paso 1: Verificar header
1. Cualquier página del sistema
2. Parte superior derecha debe mostrar nombre completo
3. Debe estar en MAYÚSCULAS
4. Ejemplo: **"OSWALDO GUEDEZ"** (NO "ADMIN")

### Paso 2: Cambiar usuario en Configuración
1. Si eres ADMIN, ve a "Configuración"
2. En "Perfil Administrador", cambia nombre a "Mi Nuevo Nombre"
3. Haz click "Guardar Cambios Admin"
4. Header debe actualizarse a "MI NUEVO NOMBRE"

---

## CONSOLE LOG ESPERADOS

Cuando todo funciona correctamente, deberías ver en consola:

```javascript
[v0] Inicializando aplicación...
[v0] Verificando sesión...
[v0] Sesión restaurada para: OSWALDO GUEDEZ
[v0] Cargando tickets...
[v0] Tickets cargados: X
[v0] Cargando configuración...
[v0] Abriendo modal ticket
[v0] Cargando dispositivos - Edificio: Edificio 01 Piso:  Área: 
[v0] Cargando pisos para edificio: Edificio 01
[v0] Pisos cargados: ['Piso 01', ...]
```

---

## SI ALGO NO FUNCIONA 🚨

### Paso 1: Abrir consola
- F12 → Tab "Console"

### Paso 2: Buscar errores
- Busca mensajes rojo (errores)
- Busca `[v0]` o `[ERROR]`
- Busca `[DIAG]`

### Paso 3: Copiar información
Copia esto en tu mensaje al desarrollador:
1. **¿Qué hiciste?** (pasos exactos)
2. **¿Qué pasó?** (descripción del error)
3. **Consola completa** (copia todo lo que veas en console)

### Paso 4: Reintentar
1. Cierra sesión
2. Presiona Ctrl+Shift+Delete (limpiar datos del navegador)
3. Abre de nuevo el sistema
4. Intenta de nuevo

---

## ERRORES COMUNES

### ❌ "La fecha de ingreso no puede ser mayor a hoy"
- **Causa**: Seleccionaste una fecha futura
- **Solución**: Usa fecha hoy o pasada

### ❌ "No hay equipos en esta ubicación"
- **Causa**: El edificio/piso/área seleccionado no tiene dispositivos
- **Solución**: Intenta otra ubicación

### ❌ Modal no cierra
- **Causa**: Botón cancelar tiene problema
- **Solución**: Actualiza consola y reporta error

### ❌ Sesión se perdió
- **Causa**: Cookies deshabilitadas en navegador
- **Solución**: Habilita cookies, o contacta desarrollador

---

## CHECKLIST FINAL ✅

- [ ] Sesión persiste después de refresh
- [ ] Nombre de usuario en MAYÚSCULAS en header
- [ ] Jerarquía funciona: Edificio → Piso → Área
- [ ] Dispositivos muestran: Tipo - Ubicación - Marca - Modelo - Serial
- [ ] Validación de fecha futura funciona
- [ ] Botones cancelar cierran modales
- [ ] Paginación funciona correctamente
- [ ] Barra de progreso muestra porcentaje correcto
- [ ] No hay errores en consola (rojo)

---

Si todo funciona ✅, ¡el sistema está listo!
