package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

type apiRoute struct {
	Method  string
	Path    string
	Summary string
	Tag     string
	Public  bool
	Admin   bool
	Body    string
	Query   []string
	Created bool
	NoAuth  bool
}

var apiRoutes = []apiRoute{
	{"GET", "/api/v1/auth/status", "Estado de autenticación, versión y marca demo", "Autenticación", true, false, "", nil, false, false},
	{"POST", "/api/v1/auth/login", "Iniciar sesión", "Autenticación", true, false, "LoginRequest", nil, false, false},
	{"POST", "/api/v1/auth/setup", "Crear la cuenta inicial de administrador", "Autenticación", true, false, "LoginRequest", nil, true, false},
	{"POST", "/api/v1/auth/logout", "Cerrar la sesión actual", "Autenticación", false, false, "", nil, false, false},
	{"POST", "/api/v1/auth/password", "Cambiar la contraseña propia", "Autenticación", false, false, "ChangePasswordRequest", nil, false, false},
	{"GET", "/api/v1/auth/users", "Listar usuarios locales", "Usuarios", false, true, "", nil, false, false},
	{"POST", "/api/v1/auth/users", "Crear un usuario", "Usuarios", false, true, "CreateUserRequest", nil, true, false},
	{"PATCH", "/api/v1/auth/users/{id}", "Cambiar el rol de un usuario", "Usuarios", false, true, "UpdateUserRequest", nil, false, false},
	{"DELETE", "/api/v1/auth/users/{id}", "Eliminar un usuario", "Usuarios", false, true, "", nil, false, false},
	{"POST", "/api/v1/auth/users/{id}/password", "Restablecer la contraseña de un usuario", "Usuarios", false, true, "ResetPasswordRequest", nil, false, false},

	{"GET", "/api/v1/dashboard", "Resumen operativo del panel", "Panel", false, false, "", []string{"site_id"}, false, false},

	{"GET", "/api/v1/sites", "Listar sedes", "Sedes", false, false, "", nil, false, false},
	{"POST", "/api/v1/sites", "Crear una sede", "Sedes", false, true, "SiteRequest", nil, true, false},
	{"GET", "/api/v1/sites/{id}", "Detalle de una sede", "Sedes", false, false, "", nil, false, false},
	{"PUT", "/api/v1/sites/{id}", "Renombrar una sede", "Sedes", false, true, "SiteRequest", nil, false, false},
	{"DELETE", "/api/v1/sites/{id}", "Eliminar una sede", "Sedes", false, true, "", []string{"move_to"}, false, false},
	{"GET", "/api/v1/sites/{id}/export", "Exportar una sede como paquete JSON", "Sedes", false, false, "", nil, false, false},
	{"POST", "/api/v1/sites/{id}/import", "Importar un paquete en una sede", "Sedes", false, false, "SiteBundle", []string{"mode"}, false, false},

	{"GET", "/api/v1/hosts", "Listar el inventario", "Dispositivos", false, false, "", []string{"site_id", "page", "limit", "search", "status", "device_type", "network_id", "favorite", "sort", "asc"}, false, false},
	{"GET", "/api/v1/hosts/export", "Exportar el inventario", "Dispositivos", false, false, "", []string{"format"}, false, false},
	{"GET", "/api/v1/hosts/{id}", "Detalle de un dispositivo", "Dispositivos", false, false, "", nil, false, false},
	{"PATCH", "/api/v1/hosts/{id}", "Editar notas, etiquetas y favorito", "Dispositivos", false, false, "UpdateHostRequest", nil, false, false},
	{"DELETE", "/api/v1/hosts/{id}", "Eliminar un dispositivo", "Dispositivos", false, false, "", nil, false, false},
	{"GET", "/api/v1/hosts/{id}/events", "Eventos de un dispositivo", "Dispositivos", false, false, "", []string{"limit"}, false, false},
	{"GET", "/api/v1/hosts/{id}/history", "Histórico de latencia y disponibilidad", "Dispositivos", false, false, "", []string{"from", "to", "range", "points"}, false, false},
	{"GET", "/api/v1/hosts/{id}/neighbors", "Vecinos conocidos de un dispositivo", "Dispositivos", false, false, "", nil, false, false},
	{"POST", "/api/v1/hosts/{id}/rescan", "Volver a escanear un dispositivo", "Dispositivos", false, false, "RescanRequest", nil, false, false},
	{"GET", "/api/v1/ports/summary", "Puertos más expuestos", "Dispositivos", false, false, "", []string{"site_id"}, false, false},
	{"POST", "/api/v1/history/compare", "Comparar dos momentos (por defecto hoy vs hace 7 días)", "Historial", false, false, "HistoryCompareRequest", []string{"site_id", "days", "window"}, false, false},
	{"POST", "/api/v1/audits/snapshot", "Guardar un snapshot de auditoría", "Auditoría", false, false, "AuditSnapshotRequest", []string{"site_id"}, true, false},
	{"GET", "/api/v1/audits", "Listar snapshots de auditoría", "Auditoría", false, false, "", []string{"site_id", "limit"}, false, false},
	{"GET", "/api/v1/audits/{id}", "Detalle de un snapshot de auditoría", "Auditoría", false, false, "", []string{"data"}, false, false},
	{"DELETE", "/api/v1/audits/{id}", "Eliminar un snapshot de auditoría", "Auditoría", false, false, "", nil, false, false},
	{"POST", "/api/v1/audits/{id}/diff/{id2}", "Comparar dos snapshots de auditoría", "Auditoría", false, false, "", nil, false, false},
	{"GET", "/api/v1/audits/{id}/export", "Exportar un snapshot (json|gzip)", "Auditoría", false, false, "", []string{"format"}, false, false},

	{"GET", "/api/v1/update/status", "Estado local de actualizaciones", "Actualizaciones", false, false, "", nil, false, false},
	{"GET", "/api/v1/update/check", "Comprobar versión firmada", "Actualizaciones", false, false, "", nil, false, false},
	{"POST", "/api/v1/update/apply", "Descargar, verificar e instalar una actualización", "Actualizaciones", false, true, "UpdateApplyRequest", nil, false, false},
	{"POST", "/api/v1/update/rollback", "Restaurar el binario anterior", "Actualizaciones", false, true, "", nil, false, false},

	{"GET", "/api/v1/networks", "Listar redes monitorizadas", "Redes", false, false, "", []string{"site_id"}, false, false},
	{"POST", "/api/v1/networks", "Añadir una red", "Redes", false, true, "NetworkRequest", nil, true, false},
	{"GET", "/api/v1/networks/suggest", "Sugerir redes locales", "Redes", false, false, "", []string{"site_id"}, false, false},
	{"PUT", "/api/v1/networks/{id}", "Editar una red", "Redes", false, true, "NetworkRequest", nil, false, false},
	{"DELETE", "/api/v1/networks/{id}", "Eliminar una red", "Redes", false, true, "", nil, false, false},

	{"GET", "/api/v1/scans", "Listar escaneos", "Escaneos", false, false, "", []string{"site_id", "limit", "status"}, false, false},
	{"POST", "/api/v1/scans", "Iniciar un escaneo", "Escaneos", false, false, "ScanRequest", nil, true, false},
	{"GET", "/api/v1/scans/{id}", "Detalle de un escaneo", "Escaneos", false, false, "", nil, false, false},
	{"POST", "/api/v1/scans/{id}/cancel", "Cancelar un escaneo", "Escaneos", false, false, "", nil, false, false},

	{"GET", "/api/v1/alerts", "Listar alertas", "Alertas", false, false, "", []string{"site_id", "status", "severity", "search", "page", "limit"}, false, false},
	{"GET", "/api/v1/alerts/stats", "Estadísticas de alertas", "Alertas", false, false, "", []string{"site_id"}, false, false},
	{"POST", "/api/v1/alerts/{id}/ack", "Reconocer una alerta", "Alertas", false, false, "", nil, false, false},
	{"POST", "/api/v1/alerts/{id}/resolve", "Resolver una alerta", "Alertas", false, false, "", nil, false, false},
	{"GET", "/api/v1/alerts/rules", "Listar reglas de alerta", "Alertas", false, false, "", nil, false, false},
	{"POST", "/api/v1/alerts/rules", "Crear una regla de alerta", "Alertas", false, true, "AlertRuleRequest", nil, true, false},
	{"PUT", "/api/v1/alerts/rules/{id}", "Editar una regla de alerta", "Alertas", false, true, "AlertRuleRequest", nil, false, false},
	{"DELETE", "/api/v1/alerts/rules/{id}", "Eliminar una regla de alerta", "Alertas", false, true, "", nil, false, false},
	{"GET", "/api/v1/alerts/channels", "Listar canales de notificación", "Alertas", false, true, "", nil, false, false},
	{"POST", "/api/v1/alerts/channels", "Crear un canal de notificación", "Alertas", false, true, "AlertChannelRequest", nil, true, false},
	{"PUT", "/api/v1/alerts/channels/{id}", "Editar un canal de notificación", "Alertas", false, true, "AlertChannelRequest", nil, false, false},
	{"DELETE", "/api/v1/alerts/channels/{id}", "Eliminar un canal de notificación", "Alertas", false, true, "", nil, false, false},
	{"POST", "/api/v1/alerts/channels/{id}/test", "Enviar una notificación de prueba", "Alertas", false, true, "", nil, false, false},

	{"GET", "/api/v1/integrations", "Listar integraciones externas", "Integraciones", false, true, "", nil, false, false},
	{"POST", "/api/v1/integrations", "Crear una integración externa", "Integraciones", false, true, "IntegrationRequest", nil, true, false},
	{"PUT", "/api/v1/integrations/{id}", "Editar una integración externa", "Integraciones", false, true, "IntegrationRequest", nil, false, false},
	{"DELETE", "/api/v1/integrations/{id}", "Eliminar una integración externa", "Integraciones", false, true, "", nil, false, false},
	{"POST", "/api/v1/integrations/{id}/test", "Probar una integración externa", "Integraciones", false, true, "", nil, false, false},

	{"GET", "/api/v1/reports", "Listar informes generados", "Informes", false, false, "", []string{"site_id"}, false, false},
	{"POST", "/api/v1/reports", "Generar un informe", "Informes", false, false, "ReportRequest", nil, true, false},
	{"GET", "/api/v1/reports/{id}/download", "Descargar un informe", "Informes", false, false, "", nil, false, false},
	{"DELETE", "/api/v1/reports/{id}", "Eliminar un informe", "Informes", false, true, "", nil, false, false},

	{"GET", "/api/v1/topology", "Grafo de topología L2/L3 con redes vecinas detectadas", "Topología", false, false, "", []string{"site_id", "network_id"}, false, false},
	{"POST", "/api/v1/trace", "Traza la ruta hasta un destino IPv4", "Topología", false, false, "TraceRequest", nil, false, false},
	{"POST", "/api/v1/discover/neighbors", "Detecta redes privadas vecinas mediante trazas", "Topología", false, false, "", nil, false, false},
	{"GET", "/api/v1/discover/neighbors", "Devuelve las redes vecinas detectadas", "Topología", false, false, "", nil, false, false},

	{"GET", "/api/v1/dashboards", "Listar dashboards personalizados", "Dashboards", false, false, "", []string{"site_id"}, false, false},
	{"GET", "/api/v1/dashboards/catalog", "Catalogo de widgets permitidos", "Dashboards", false, false, "", nil, false, false},
	{"POST", "/api/v1/dashboards", "Crear dashboard personalizado", "Dashboards", false, false, "DashboardRequest", nil, true, false},
	{"POST", "/api/v1/dashboards/example", "Crear dashboard de ejemplo Mi red", "Dashboards", false, false, "", nil, false, false},
	{"GET", "/api/v1/dashboards/{id}", "Obtener un dashboard", "Dashboards", false, false, "", nil, false, false},
	{"PUT", "/api/v1/dashboards/{id}", "Actualizar un dashboard", "Dashboards", false, false, "DashboardRequest", nil, true, false},
	{"DELETE", "/api/v1/dashboards/{id}", "Eliminar un dashboard", "Dashboards", false, false, "", nil, false, false},

	{"GET", "/api/v1/views", "Listar las vistas guardadas del usuario", "Vistas", false, false, "", []string{"kind", "site_id"}, false, false},
	{"POST", "/api/v1/views", "Guardar una vista de inventario o alertas", "Vistas", false, false, "SavedViewRequest", nil, true, false},
	{"GET", "/api/v1/views/catalog", "Catálogo de columnas, órdenes y vistas por defecto", "Vistas", false, false, "", nil, false, false},
	{"GET", "/api/v1/views/resolve", "Validar y resolver los parámetros de un enlace compartido", "Vistas", false, false, "", []string{"name", "kind", "site_id", "q", "severity", "status", "window", "columns", "sort", "desc"}, false, false},
	{"GET", "/api/v1/views/{id}", "Obtener una vista propia", "Vistas", false, false, "", nil, false, false},
	{"PUT", "/api/v1/views/{id}", "Actualizar una vista propia", "Vistas", false, false, "SavedViewRequest", nil, false, false},
	{"DELETE", "/api/v1/views/{id}", "Eliminar una vista propia", "Vistas", false, false, "", nil, false, false},
	{"GET", "/api/v1/views/{id}/share", "Obtener la URL con parámetros para compartir una vista", "Vistas", false, false, "", nil, false, false},

	{"GET", "/api/v1/wifi/status", "Estado y capacidad de captura WiFi", "WiFi", false, false, "", nil, false, false},
	{"GET", "/api/v1/wifi/aps", "Listar APs WiFi observados", "WiFi", false, false, "", []string{"site_id", "limit"}, false, false},
	{"GET", "/api/v1/wifi/clients", "Listar clientes WiFi observados", "WiFi", false, false, "", []string{"site_id", "limit"}, false, false},
	{"GET", "/api/v1/wifi/channels", "Analizar ocupación e interferencia por canal", "WiFi", false, false, "", []string{"site_id"}, false, false},
	{"POST", "/api/v1/wifi/scan", "Capturar beacons y clientes durante una ventana", "WiFi", false, false, "", []string{"site_id"}, false, false},
	{"DELETE", "/api/v1/wifi/aps", "Borrar observaciones WiFi de una sede", "WiFi", false, true, "", []string{"site_id"}, false, false},
	{"GET", "/api/v1/vpn/tunnels", "Listar túneles VPN detectados", "VPN", false, false, "", []string{"site_id"}, false, false},
	{"POST", "/api/v1/vpn/scan", "Observar túneles VPN ahora", "VPN", false, false, "", []string{"site_id"}, false, false},
	{"DELETE", "/api/v1/vpn/tunnels/{id}", "Eliminar una observación VPN", "VPN", false, true, "", nil, false, false},
	{"GET", "/api/v1/diagnostics/enterprise", "Diagnósticos empresariales heurísticos", "Diagnósticos", false, false, "", []string{"site_id", "from", "to"}, false, false},
	{"GET", "/api/v1/diagnostics/stp", "Posibles lazos de spanning-tree", "Diagnósticos", false, false, "", []string{"site_id", "from", "to"}, false, false},
	{"GET", "/api/v1/diagnostics/vlan", "Posibles fugas de VLAN", "Diagnósticos", false, false, "", []string{"site_id", "from", "to"}, false, false},
	{"GET", "/api/v1/diagnostics/arp", "Anomalías ARP y histórico IP→MAC", "Diagnósticos", false, false, "", []string{"site_id", "from", "to"}, false, false},

	{"GET", "/api/v1/system/info", "Información del sistema, capacidades y marca demo", "Sistema", false, false, "", nil, false, false},
	{"GET", "/api/v1/system/license", "Estado de la licencia offline y avisos", "Sistema", false, false, "", nil, false, false},
	{"GET", "/api/v1/system/interfaces", "Interfaces de red locales", "Sistema", false, false, "", nil, false, false},
	{"GET", "/api/v1/system/config", "Leer la configuración", "Sistema", false, true, "", nil, false, false},
	{"PUT", "/api/v1/system/config", "Guardar la configuración", "Sistema", false, true, "Config", nil, false, false},
	{"GET", "/api/v1/system/audit", "Consultar la auditoría", "Sistema", false, true, "", []string{"limit", "actor", "action", "from", "to"}, false, false},
	{"GET", "/api/v1/system/health", "Comprobación de salud", "Sistema", true, false, "", nil, false, false},
	{"GET", "/api/v1/system/backups", "Listar copias de seguridad", "Sistema", false, true, "", nil, false, false},
	{"POST", "/api/v1/system/backups", "Crear una copia de seguridad", "Sistema", false, true, "", nil, true, false},
	{"GET", "/api/v1/system/backups/{name}/download", "Descargar una copia de seguridad", "Sistema", false, true, "", nil, false, false},
	{"DELETE", "/api/v1/system/backups/{name}", "Eliminar una copia de seguridad", "Sistema", false, true, "", nil, false, false},
	{"GET", "/api/v1/ws", "Canal WebSocket de eventos en vivo", "Tiempo real", false, false, "", nil, false, true},
	{"GET", "/api/v1/openapi.json", "Esta especificación OpenAPI", "Sistema", true, false, "", nil, false, true},
	{"GET", "/metrics", "Métricas en formato Prometheus", "Sistema", true, false, "", []string{"site_id"}, false, true},
}

func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	spec := buildOpenAPI(s.version, s.commit, s.buildDate)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(spec)
}

func buildOpenAPI(version, commit, buildDate string) map[string]any {
	paths := map[string]any{}
	for _, rt := range apiRoutes {
		op := map[string]any{
			"summary":     rt.Summary,
			"tags":        []string{rt.Tag},
			"operationId": operationID(rt.Method, rt.Path),
			"responses": map[string]any{
				"200": map[string]any{"description": "Operación correcta"},
				"400": map[string]any{"description": "Petición inválida", "content": jsonContent("Error")},
				"401": map[string]any{"description": "No autenticado", "content": jsonContent("Error")},
				"403": map[string]any{"description": "Permiso insuficiente o CSRF inválido", "content": jsonContent("Error")},
				"429": map[string]any{"description": "Demasiadas peticiones", "content": jsonContent("Error")},
				"500": map[string]any{"description": "Error interno", "content": jsonContent("Error")},
			},
			"security": []map[string]any{},
		}
		if rt.Created {
			op["responses"].(map[string]any)["201"] = map[string]any{"description": "Recurso creado"}
		}
		if rt.Body != "" {
			op["requestBody"] = map[string]any{
				"required": true,
				"content":  jsonContent(rt.Body),
			}
		}
		if len(rt.Query) > 0 {
			params := make([]map[string]any, 0, len(rt.Query))
			for _, q := range rt.Query {
				params = append(params, map[string]any{
					"name": q, "in": "query", "required": false,
					"schema": map[string]any{"type": "string"},
				})
			}
			op["parameters"] = params
		}
		for _, p := range pathParams(rt.Path) {
			op["parameters"] = appendParam(op["parameters"], map[string]any{
				"name": p, "in": "path", "required": true,
				"schema": map[string]any{"type": "string"},
			})
		}
		if !rt.Public {
			op["security"] = []map[string]any{{"session": map[string]any{}}}
			if isMutating(rt.Method) {
				op["parameters"] = appendParam(op["parameters"], map[string]any{
					"name": "X-CSRF-Token", "in": "header", "required": true,
					"schema":      map[string]any{"type": "string"},
					"description": "Token CSRF devuelto por /auth/status o /auth/login",
				})
			}
		}
		if rt.Admin {
			op["description"] = "Requiere rol admin u owner."
		}
		item, _ := paths[rt.Path].(map[string]any)
		if item == nil {
			item = map[string]any{}
			paths[rt.Path] = item
		}
		item[strings.ToLower(rt.Method)] = op
	}
	return map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":       "NetDiag API",
			"version":     version,
			"description": "API REST del escáner de red y monitor de inventario NetDiag. Las mutaciones requieren sesión, permiso de escritura y cabecera X-CSRF-Token.",
			"contact":     map[string]any{"name": "NetDiag"},
			"x-build":     map[string]any{"commit": commit, "date": buildDate},
		},
		"servers": []map[string]any{{"url": "/", "description": "Este servidor"}},
		"tags": []map[string]any{
			{"name": "Autenticación"}, {"name": "Usuarios"}, {"name": "Panel"},
			{"name": "Sedes"}, {"name": "Dispositivos"}, {"name": "Redes"}, {"name": "Escaneos"},
			{"name": "Alertas"}, {"name": "Vistas"}, {"name": "Integraciones"}, {"name": "Informes"}, {"name": "Topología"},
			{"name": "Diagnósticos"}, {"name": "Actualizaciones"}, {"name": "Sistema"}, {"name": "Tiempo real"},
		},
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"session": map[string]any{
					"type": "apiKey", "in": "cookie", "name": "netdiag_session",
					"description": "Cookie de sesión opaca creada por /auth/login.",
				},
			},
			"schemas": openAPISchemas(),
		},
		"paths": paths,
	}
}

func jsonContent(schema string) map[string]any {
	return map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/" + schema}}}
}

func appendParam(existing any, p map[string]any) []map[string]any {
	params, _ := existing.([]map[string]any)
	return append(params, p)
}

func pathParams(path string) []string {
	var out []string
	for {
		start := strings.Index(path, "{")
		if start < 0 {
			return out
		}
		end := strings.Index(path[start:], "}")
		if end < 0 {
			return out
		}
		out = append(out, path[start+1:start+end])
		path = path[start+end+1:]
	}
}

func operationID(method, path string) string {
	repl := strings.NewReplacer("/api/v1/", "", "/", "_", "{", "", "}", "")
	return strings.ToLower(method) + "_" + strings.Trim(repl.Replace(path), "_")
}

func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

func openAPISchemas() map[string]any {
	str := map[string]any{"type": "string"}
	strArray := map[string]any{"type": "array", "items": str}
	return map[string]any{
		"Error": map[string]any{
			"type": "object", "required": []string{"error"},
			"properties": map[string]any{
				"error": str, "code": str, "details": str,
			},
		},
		"LicenseStatus": map[string]any{
			"type": "object", "required": []string{"status", "edition", "limits", "warnings"},
			"properties": map[string]any{
				"status": str, "edition": str, "license_id": str, "expires_at": str,
				"support_level": str, "capabilities": strArray,
				"managed_hosts": map[string]any{"type": "integer"},
				"sites":         map[string]any{"type": "integer"},
				"limits": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"max_managed_hosts": map[string]any{"type": "integer"},
						"max_sites":         map[string]any{"type": "integer"},
						"retention_days":    map[string]any{"type": "integer"},
					},
				},
				"warnings": map[string]any{"type": "array", "items": str},
			},
		},
		"LoginRequest": map[string]any{
			"type": "object", "required": []string{"username", "password"},
			"properties": map[string]any{"username": str, "password": str},
		},
		"ChangePasswordRequest": map[string]any{
			"type": "object", "required": []string{"current_password", "new_password"},
			"properties": map[string]any{"current_password": str, "new_password": str},
		},
		"CreateUserRequest": map[string]any{
			"type": "object", "required": []string{"username", "password", "role"},
			"properties": map[string]any{
				"username": str, "password": str,
				"role": map[string]any{"type": "string", "enum": []string{"admin", "operator", "viewer"}},
			},
		},
		"UpdateUserRequest": map[string]any{
			"type": "object", "required": []string{"role"},
			"properties": map[string]any{
				"role": map[string]any{"type": "string", "enum": []string{"admin", "operator", "viewer"}},
			},
		},
		"ResetPasswordRequest": map[string]any{
			"type": "object", "required": []string{"password"},
			"properties": map[string]any{"password": str},
		},
		"UpdateHostRequest": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"hostname": str, "notes": str, "favorite": map[string]any{"type": "boolean"},
				"tags": strArray,
			},
		},
		"RescanRequest": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"profile": map[string]any{"type": "string", "enum": []string{"quick", "common", "full", "custom"}},
				"snmp":    map[string]any{"type": "boolean"},
				"deep":    map[string]any{"type": "boolean"},
			},
		},
		"SiteRequest": map[string]any{
			"type": "object", "required": []string{"name"},
			"properties": map[string]any{"name": str, "description": str},
		},
		"SiteBundle": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"schema": str, "exported_at": str,
				"site":     map[string]any{"type": "object"},
				"networks": map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
				"hosts":    map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
			},
		},
		"NetworkRequest": map[string]any{
			"type": "object", "required": []string{"cidr"},
			"properties": map[string]any{
				"site_id": map[string]any{"type": "integer", "format": "int64"},
				"name":    str, "cidr": str, "interface": str,
				"enabled": map[string]any{"type": "boolean"},
			},
		},
		"ScanRequest": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"site_id":      map[string]any{"type": "integer", "format": "int64"},
				"network_id":   map[string]any{"type": "integer", "format": "int64"},
				"cidr":         str,
				"profile":      map[string]any{"type": "string", "enum": []string{"quick", "common", "full", "custom"}},
				"ports":        map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
				"snmp":         map[string]any{"type": "boolean"},
				"deep":         map[string]any{"type": "boolean"},
				"timeout_ms":   map[string]any{"type": "integer"},
				"allow_public": map[string]any{"type": "boolean"},
			},
		},
		"AlertRuleRequest": map[string]any{
			"type": "object", "required": []string{"name", "type"},
			"properties": map[string]any{
				"name": str, "type": str, "enabled": map[string]any{"type": "boolean"},
				"severity": str, "config": map[string]any{"type": "object"},
				"channel_ids": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
			},
		},
		"AlertChannelRequest": map[string]any{
			"type": "object", "required": []string{"name", "type"},
			"properties": map[string]any{
				"name": str, "type": str, "enabled": map[string]any{"type": "boolean"},
				"config": map[string]any{"type": "object", "additionalProperties": true},
			},
		},
		"IntegrationRequest": map[string]any{
			"type": "object", "required": []string{"name", "type"},
			"properties": map[string]any{
				"name": str, "enabled": map[string]any{"type": "boolean"},
				"type": map[string]any{"type": "string", "enum": []string{
					"elasticsearch", "syslog", "webhook", "slack", "discord", "msteams",
					"telegram", "pagerduty", "kafka", "redis",
				}},
				"config": map[string]any{"type": "object", "additionalProperties": true},
			},
		},
		"DashboardRequest": map[string]any{
			"type": "object", "required": []string{"name"},
			"properties": map[string]any{
				"name":    str,
				"site_id": map[string]any{"type": "integer"},
				"widgets": map[string]any{"type": "array", "items": map[string]any{
					"type": "object", "properties": map[string]any{
						"id": str, "type": str,
						"position": map[string]any{"type": "object"},
						"size":     map[string]any{"type": "object"},
						"config":   map[string]any{"type": "object", "additionalProperties": true},
					},
				}},
			},
		},
		"SavedViewRequest": map[string]any{
			"type": "object", "required": []string{"name", "kind"},
			"properties": map[string]any{
				"name": str, "kind": map[string]any{"type": "string", "enum": []string{"hosts", "alerts"}},
				"site_id":  map[string]any{"type": "integer", "description": "0 = todas las sedes"},
				"query":    map[string]any{"type": "string", "maxLength": 200},
				"severity": map[string]any{"type": "string", "enum": []string{"", "info", "warning", "critical"}},
				"status":   str, "window": str,
				"columns": map[string]any{"type": "array", "items": str, "maxItems": 16},
				"sort":    str, "desc": map[string]any{"type": "boolean"},
			},
		},
		"SavedView": map[string]any{
			"type": "object", "required": []string{"id", "name", "kind", "columns"},
			"properties": map[string]any{
				"id": map[string]any{"type": "integer"}, "name": str,
				"kind":    map[string]any{"type": "string", "enum": []string{"hosts", "alerts"}},
				"site_id": map[string]any{"type": "integer"}, "query": str,
				"severity": str, "status": str, "window": str,
				"columns": map[string]any{"type": "array", "items": str},
				"sort":    str, "desc": map[string]any{"type": "boolean"},
				"builtin":    map[string]any{"type": "boolean"},
				"created_at": map[string]any{"type": "string", "format": "date-time"},
				"updated_at": map[string]any{"type": "string", "format": "date-time"},
			},
		},
		"ViewShare": map[string]any{
			"type": "object", "required": []string{"path", "params"},
			"properties": map[string]any{
				"id": map[string]any{"type": "integer"}, "name": str,
				"path": str, "url": str,
				"params": map[string]any{"type": "object", "additionalProperties": true},
			},
		},
		"AuditSnapshotRequest": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"site_id": str,
				"name":    str,
				"notes":   str,
			},
		},
		"HistoryCompareRequest": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"site_id": map[string]any{"type": "integer"},
				"days":    map[string]any{"type": "integer", "default": 7},
				"window":  map[string]any{"type": "string", "default": "24h"},
				"period_a": map[string]any{"type": "object", "properties": map[string]any{
					"label": str, "from": str, "to": str,
				}},
				"period_b": map[string]any{"type": "object", "properties": map[string]any{
					"label": str, "from": str, "to": str,
				}},
			},
		},
		"UpdateApplyRequest": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"version": map[string]any{"type": "string", "description": "Versión esperada; debe coincidir con el manifiesto firmado"},
				"restart": map[string]any{"type": "boolean", "default": true, "description": "Reinicia automáticamente tras responder"},
			},
		},
		"UpdateCheckResult": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"current_version": str, "latest_version": str,
				"available":       map[string]any{"type": "boolean"},
				"security_update": map[string]any{"type": "boolean", "description": "Versión instalada anterior a min_version; emite el evento update.security"},
				"min_version":     map[string]any{"type": "string", "description": "Primera versión segura según el manifiesto firmado (F41)"},
				"checked_at":      map[string]any{"type": "string", "format": "date-time"},
				"message":         str,
			},
		},
		"UpdateApplyResult": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"from_version": str, "to_version": str,
				"installed_at": map[string]any{"type": "string", "format": "date-time"},
				"backup_path":  str, "message": str,
			},
		},
		"ReportRequest": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"site_id": map[string]any{"type": "integer", "format": "int64"},
				"kind":    map[string]any{"type": "string", "enum": []string{"inventory", "executive", "alerts"}},
				"format":  map[string]any{"type": "string", "enum": []string{"pdf", "csv", "json", "html"}},
				"title":   str,
			},
		},
		"TraceRequest": map[string]any{
			"type":     "object",
			"required": []string{"target"},
			"properties": map[string]any{
				"target":     str,
				"hops":       map[string]any{"type": "integer", "minimum": 1, "maximum": 64},
				"probes":     map[string]any{"type": "integer", "minimum": 1, "maximum": 5},
				"timeout_ms": map[string]any{"type": "integer", "minimum": 50, "maximum": 5000},
				"resolve":    map[string]any{"type": "boolean"},
			},
		},
		"Config": map[string]any{"type": "object", "additionalProperties": true},
	}
}
