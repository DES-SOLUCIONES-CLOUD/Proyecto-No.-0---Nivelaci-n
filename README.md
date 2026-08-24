# 📦 OKF Platform — Plataforma de Conversión Documental a Bundles OKF

**ISIS4426 Desarrollo de Soluciones Cloud — Proyecto de Nivelación**

Plataforma web multiusuario que convierte documentos a **bundles de conocimiento** en formato OKF (Open Knowledge Format). El backend está implementado en Go, el procesamiento es completamente asíncrono mediante workers, y el sistema se despliega con un solo comando Docker Compose.

---

## 🚀 Despliegue rápido (un solo comando)

El único comando requerido para levantar el sistema completo es `docker compose up --build`. **No es necesario crear ningún archivo `.env` ni exportar variables**: todas las variables de entorno tienen valores por defecto funcionales definidos directamente en `docker-compose.yml`.

```bash
# Clona el repositorio
git clone <url-del-repositorio>
cd Proyecto-No.-0---Nivelaci-n   # o el nombre de la carpeta donde clonaste el repo

# ¡Levanta todo el sistema! (sin pasos previos)
docker compose up --build
```

Accede a la plataforma en: **http://localhost**

> 💡 **Personalizar la configuración (opcional):** si quieres usar credenciales distintas a las de desarrollo, copia `cp .env.example .env`, edita los valores y vuelve a ejecutar `docker compose up --build`. Docker Compose carga automáticamente el `.env` de la raíz del proyecto y sus valores sobrescriben los defaults. Ver la tabla de variables más abajo.

---

## 🏗️ Arquitectura

```
Browser
  │
  ▼
Nginx :80 (Frontend HTML/CSS/JS + proxy reverso)
  │
  ├── GET /          → Sirve archivos estáticos
  └── /api/*         → API Go (puerto 8080, sin estado)
                            │
                   ┌────────┴────────┐
                   │                 │
              PostgreSQL         Redis Streams
              (metadatos)        (cola de mensajes)
                   │                 │
              MinIO              Workers Go
              (objetos)          (conversión + bundle)
                   │                 │
                   └────────┬────────┘
                            │
                     PostgreSQL + MinIO
                     (resultado del bundle)
```

### Servicios Docker

| Servicio | Imagen | Puerto | Descripción |
|---|---|---|---|
| `frontend` | nginx:alpine | 80 | Frontend + proxy a la API |
| `api` | Go (multi-stage) | 8080 (interno) | API REST sin estado |
| `worker` | Go (multi-stage) | — | Worker de conversión |
| `postgres` | postgres:16-alpine | 5432 (interno) | Base de datos de metadatos |
| `redis` | redis:7-alpine | 6379 (interno) | Cola de mensajes |
| `minio` | minio/minio | 9001 (consola) | Almacenamiento de objetos |

---

## 📋 Funcionalidades

### Mínimo obligatorio ✅
- [x] Registro y autenticación con JWT
- [x] Carga de documentos con respuesta inmediata (job_id)
- [x] Encolamiento en Redis Streams + workers independientes
- [x] Segmentación por encabezados (Markdown, HTML, texto plano, PDF, DOCX)
- [x] Generación de bundle: `index.md`, `log.md`, conceptos
- [x] Validación de bundle antes de publicación
- [x] Consulta de estado y descarga del bundle (ZIP)
- [x] Aislamiento por propietario en todas las operaciones
- [x] Persistencia en MinIO (no en disco efímero)
- [x] Despliegue con un solo comando

### Formatos de entrada soportados

El enunciado exigía soportar al menos un formato con estructura detectable (encabezados). La implementación va más allá del mínimo y soporta **cinco formatos**, incluyendo dos formatos binarios (PDF, DOCX) que requieren extracción de contenido antes de segmentar.

| Formato | Segmentación |
|---|---|
| Markdown (`.md`) | Por encabezados `#`, `##`, `###` |
| HTML (`.html`, `.htm`) | Por etiquetas `<h1>`, `<h2>`, `<h3>` |
| Texto plano (`.txt`) | Por MAYÚSCULAS, líneas con ":", bloques |
| PDF (`.pdf`) | Texto extraído con `pdftotext` (poppler-utils) en orden de lectura. Se agrupan las páginas bajo el encabezado numerado que las inicia (p. ej. `1. Introducción`, `12) Conclusiones`); si el documento no tiene encabezados numerados, se genera una unidad por página fusionando las páginas muy cortas. Un PDF sin texto seleccionable (escaneado) produce un único concepto de aviso en vez de fallar. |
| Word (`.docx`) | Párrafos extraídos del XML interno (`word/document.xml`) y agrupados por los estilos de encabezado de Word (`Heading1/2/3`, `Título`, `H1`/`H2`/`H3`); si el documento no usa estilos de encabezado, todo el contenido se publica como un único concepto. |

---

## 🔗 API Endpoints

| Método | Ruta | Auth | Descripción |
|---|---|---|---|
| POST | `/api/auth/register` | No | Registrar usuario |
| POST | `/api/auth/login` | No | Login → JWT token |
| GET | `/api/auth/me` | Sí | Usuario actual |
| POST | `/api/documents` | Sí | Subir documento |
| GET | `/api/jobs` | Sí | Listar mis trabajos |
| GET | `/api/jobs/:id` | Sí | Estado de un trabajo |
| GET | `/api/jobs/:id/download` | Sí | Descargar bundle ZIP |
| GET | `/api/health` | No | Health check |

---

## 🔧 Variables de configuración

Todas las variables tienen un valor por defecto embebido en `docker-compose.yml` (sintaxis `${VAR:-default}`), por lo que el sistema arranca sin ningún archivo `.env`. Si quieres personalizarlas, copia `.env.example` a `.env`, edita lo que necesites y vuelve a ejecutar `docker compose up --build` — Compose carga automáticamente el `.env` de la raíz del proyecto y sus valores sobrescriben los defaults.

| Variable | Valor por defecto | Propósito |
|---|---|---|
| `POSTGRES_USER` | `okf` | Usuario de PostgreSQL |
| `POSTGRES_PASSWORD` | `okfpassword` | Contraseña de PostgreSQL |
| `POSTGRES_DB` | `okf` | Nombre de la base de datos |
| `POSTGRES_URL` | `postgres://okf:okfpassword@postgres:5432/okf?sslmode=disable` | Cadena de conexión usada por `api` y `worker` (debe coincidir con las tres variables anteriores) |
| `REDIS_URL` | `redis://redis:6379` | Conexión a Redis (cola de mensajes) |
| `MINIO_ENDPOINT` | `minio:9000` | Host:puerto interno de MinIO |
| `MINIO_ACCESS_KEY` | `minioadmin` | Access key de MinIO (= `MINIO_ROOT_USER`) |
| `MINIO_SECRET_KEY` | `minioadmin123` | Secret key de MinIO (= `MINIO_ROOT_PASSWORD`) |
| `MINIO_BUCKET_ORIGINALS` | `originals` | Bucket para documentos originales |
| `MINIO_BUCKET_BUNDLES` | `bundles` | Bucket para bundles generados |
| `MINIO_USE_SSL` | `false` | Si la conexión a MinIO usa TLS |
| `JWT_SECRET` | `insecure-dev-jwt-secret-CHANGE-ME-in-production` | Firma de tokens JWT |
| `PORT` | `8080` | Puerto interno del servidor de la API |
| `GIN_MODE` | `release` | Modo del framework Gin (`release` silencia el banner de debug) |

> **⚠️ Importante — `JWT_SECRET`:** el valor por defecto es **inseguro a propósito** y solo sirve para desarrollo local. En cualquier despliegue real (staging, producción, evaluación con datos sensibles) debes definir tu propio `JWT_SECRET` mediante `.env`. Nunca subas un `.env` con credenciales reales al repositorio (ya está excluido en `.gitignore`).

---

## 📁 Estructura del bundle OKF

```
bundle.zip/
├── index.md       ← Navegación y metadatos del bundle
├── log.md         ← Trazabilidad completa de la conversión
├── concepto-01.md ← Primera unidad lógica detectada
├── concepto-02.md ← Segunda unidad lógica
└── ...
```

El `index.md` enlaza todos los conceptos en orden. El `log.md` registra cada operación de conversión, validaciones y transformaciones aplicadas.

---

## 🔒 Condiciones verificables

| Condición | Implementación |
|---|---|
| **Asincronía efectiva** | API publica en Redis y retorna < 100ms. El cliente puede cerrar la conexión. |
| **Documento breve** | Un texto sin encabezados genera 1 concepto + index.md + log.md |
| **Documento estructurado** | N encabezados → N conceptos enlazados en index.md |
| **Bundle incompleto** | Si falta index.md o log.md: validación falla, no se publica |
| **Aislamiento** | Verificación de user_id en cada endpoint; 404 sin revelar información |
| **Sin duplicados** | Redis consumer groups + tabla `job_idempotency` en PostgreSQL |

---

## 🧪 Pruebas manuales

### 1. Verificar despliegue
```bash
curl http://localhost/api/health
# → {"status":"ok"}
```

### 2. Registrar usuario
```bash
curl -X POST http://localhost/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"test","email":"test@test.com","password":"password123"}'
```

### 3. Subir documento
```bash
# Guardar token del login
TOKEN="<jwt-token>"

curl -X POST http://localhost/api/documents \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@mi-documento.md"
# → {"job_id":"...","status":"pending","message":"..."}
```

### 4. Consultar estado
```bash
curl http://localhost/api/jobs/<job-id> \
  -H "Authorization: Bearer $TOKEN"
```

### 5. Descargar bundle
```bash
curl -o bundle.zip http://localhost/api/jobs/<job-id>/download \
  -H "Authorization: Bearer $TOKEN"
```

### 6. Probar aislamiento
```bash
# Con token de usuario B, intentar acceder a job de usuario A
curl http://localhost/api/jobs/<job-id-de-A> \
  -H "Authorization: Bearer $TOKEN_B"
# → 404 Not Found (no revela que el recurso existe)
```

---

## 📂 Estructura del repositorio

```
Proyecto-No.-0---Nivelaci-n/
├── docker-compose.yml
├── .env.example
├── README.md
├── api/           ← Backend API en Go (Gin)
│   ├── Dockerfile
│   ├── main.go
│   ├── config/, db/, models/, handlers/, middleware/, queue/, storage/
├── worker/        ← Worker de conversión en Go
│   ├── Dockerfile
│   ├── main.go
│   ├── converter/, bundle/, queue/, storage/
├── frontend/      ← Frontend HTML/CSS/JS servido por Nginx
│   ├── Dockerfile, nginx.conf
│   ├── index.html
│   ├── css/style.css, js/app.js
└── db/
    └── init.sql   ← Schema PostgreSQL
```

---

## 🤖 Uso de agentes de IA

El diseño, la codificación, la estructura arquitectural y la documentación de este proyecto se desarrollaron con el apoyo del agente de inteligencia artificial **Antigravity (Google DeepMind)**, de acuerdo con las condiciones del proyecto que permiten el uso abierto de herramientas de IA. El uso se declara en la sustentación del video.

---

## 📄 Licencia

Proyecto académico — ISIS4426 Desarrollo de Soluciones Cloud — Universidad de los Andes