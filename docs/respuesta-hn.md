# Respuesta al comentario de HN

Hilo: <https://news.ycombinator.com/item?id=50545732>
Comentario: `bradknowles`, 19/11/2026.

> This repo appears to be in Spanish. I didn't see any obvious way to switch the language to English. I don't trust online translators, so does anyone know of an alternative way to get an English language version of this repo?

## Texto listo para publicar

```text
Fair point — thanks for saying it. The web UI already has a one-click ES/EN switch (in Settings), but the repository docs were Spanish-only, which is easy to miss. I've just added README.en.md with an English overview and quick start, linked from the top of the main README.

To try it: `make build && ./bin/netdiag serve`, then open http://127.0.0.1:8080 and switch the language in Settings. If anything is still unclear, tell me which screen or doc you were looking at and I'll translate that too.
```

## Nota interna

- No es un caso de uso ni una instalación: cuenta como **señal de interés**, no como evidencia F42.
- La UI ya era bilingüe; lo que faltaba era documentación en inglés. F48 cierra esa brecha.
- Publicar solo si el usuario lo autoriza; no se hace push automático.
