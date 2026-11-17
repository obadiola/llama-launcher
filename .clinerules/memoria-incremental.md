---
description: Protocolo de memoria externa persistente (CEREBRO.md) para evitar pérdida de contexto y trabajo duplicado en modelos locales.
alwaysApply: true
---

# Protocolo de Memoria Externa (`CEREBRO.md`)

Operas con una ventana de contexto finita. Para garantizar continuidad absoluta entre reinicios, fallos de caché o cambios de sesión, debes mantener un "cerebro externo" en el archivo `CEREBRO.md` en la raíz del proyecto.

## Regla de Oro: Pensar en disco, no en memoria volátil
Antes de escribir o modificar cualquier archivo de código fuente, cada vez que concibas una función, estructura de datos, contrato de API o decisión técnica, **debes registrarla primero en `CEREBRO.md`**.

## Estructura obligatoria de `CEREBRO.md`
Mantén el archivo actualizado con exactamente estas 4 secciones compactas:

1. **Estado Actual y Tareas (`[ ]` pendiente / `[x]` completado):** Lista atómica de pasos.
2. **Arquitectura y Contratos (Firmas aprobadas):** Lista de archivos y las firmas exactas de funciones/structs ya diseñadas o implementadas (sin cuerpos de función, solo firmas y breve propósito de 1 línea).
3. **Banco de Ideas / Funciones Pendientes de Implementar:** Cualquier función auxiliar, edge-case o mejora que hayas pensado incorporar pero aún no hayas escrito en código.
4. **Decisiones Técnicas y Restricciones:** Reglas clave (ej. "No usar dependencias externas", "Claves JSON dinámicas").

## Flujo de Trabajo Obligatorio en cada turno

1. **Al iniciar o reanudar:** Lee `CEREBRO.md` antes que ningún otro archivo. No leas archivos de código ya marcados como `[x]` a menos que necesites editarlos, confía en las firmas registradas en `CEREBRO.md`.
2. **Al diseñar:** Si decides crear una nueva función o cambiar una lógica, actualiza primero `CEREBRO.md` añadiendo su firma en *Arquitectura y Contratos* o *Banco de Ideas*.
3. **Al implementar:** Escribe **un solo archivo o función principal por paso**. Guárdalo en disco inmediatamente.
4. **Al cerrar el paso:** Marca la tarea como `[x]` en `CEREBRO.md`, mueve las funciones ya escritas del *Banco de Ideas* a *Arquitectura y Contratos* con su firma definitiva, y guarda `CEREBRO.md`.