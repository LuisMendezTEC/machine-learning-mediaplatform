# Guía de Desarrollo — Análisis Multiproceso y Distribuido de Mensajería
**Base:** `mediaplatform` (Go) adaptado para análisis ML  
**Fecha límite:** 10 de junio de 2026  
**Equipo:** 3 integrantes — A (Infraestructura), B (Workers ML), C (API/Dashboard/Reportes)

---

## 1. Resumen de Arquitectura

```
Cliente / Dashboard
        │
        ▼
 Coordinator (Go) ←──────── WebSocket ──────────► Dashboard React
        │
        ├── Redis Streams (high / normal / low)
        │
        ├──► Python Worker: Text   (spaCy + HuggingFace)
        ├──► Python Worker: Image  (YOLOv8 + OpenCV)
        └──► Python Worker: Audio  (Whisper + NLP)
                │
                ▼
          PostgreSQL (cases, jobs, findings, evidence)
                │
                ▼
         Report Engine (Python)
```

El coordinator Go **no cambia de arquitectura** — sigue siendo el hub central.  
Los workers nuevos son microservicios Python que implementan el mismo contrato HTTP.

---

## 2. Contrato de Interfaz (firmar en Semana 1 — TODOS deben acordar)

Este es el documento de contrato que desbloquea el trabajo paralelo.  
Nadie puede romperlo sin notificar al equipo.

### 2.1 Registro de Worker (A define → B implementa)

```
POST /workers/register
Body: { "id": "text-worker-1", "hostname": "text-worker-1:8090" }
Response 200: { "status": "registered" }
```

### 2.2 Asignación de Job (A define → B implementa)

```
POST /tasks
Body: {
  "id": "<uuid>",
  "file_path": "/app/dataset/files/chat_export.txt",
  "operation": "analyze_text" | "analyze_image" | "analyze_audio",
  "priority": 1-10,
  "case_id": "<uuid>"   ← NUEVO campo
}
Response 202: { "status": "accepted" }
Response 429: { "error": "worker pool full" }
```

### 2.3 Reporte de Progreso (B implementa → A ya tiene el endpoint)

```
POST /jobs/{id}/progress
Body: { "progress": 0-100, "status": "running" | "completed" | "failed", "error": "..." }
```

### 2.4 Schema de Finding (B define → C consume)

```json
{
  "job_id": "<uuid>",
  "case_id": "<uuid>",
  "worker_type": "text" | "image" | "audio",
  "category": "keyword" | "sentiment" | "violence" | "offensive" | "weapon" | "threat",
  "confidence": 0.0-1.0,
  "risk_level": "low" | "medium" | "high" | "critical",
  "evidence": {
    "text_fragment": "...",
    "timestamp": "00:01:23",
    "keywords": ["palabra1", "palabra2"],
    "bounding_box": { "x": 0, "y": 0, "w": 100, "h": 100 },
    "transcription": "texto transcrito..."
  }
}
```

### 2.5 Operaciones válidas (A define en Go models)

| Operation | Worker responsable | Input esperado |
|---|---|---|
| `analyze_text` | text-worker | .txt, .json (export de chat) |
| `analyze_image` | image-worker | .jpg, .png, .webp |
| `analyze_audio` | audio-worker | .mp3, .wav, .ogg, .m4a |

---

## 3. INTEGRANTE A — Infraestructura y Orquestación

> **Lenguaje:** Go (coordinator existente) + YAML (Docker Compose)  
> **Copiloto:** Claude

### 3.1 Tareas ordenadas cronológicamente

#### SEMANA 1 — Fundamentos (días 1-5)

**Tarea A-1: Actualizar el schema de base de datos**

Archivo: `internal/db/db.go` función `Migrate()`

Añadir las siguientes tablas al final del SQL existente:

```sql
CREATE TABLE IF NOT EXISTS cases (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    status      TEXT NOT NULL DEFAULT 'queued',
    priority    INT  NOT NULL DEFAULT 5,
    risk_score  FLOAT DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS findings (
    id           TEXT PRIMARY KEY,
    case_id      TEXT NOT NULL REFERENCES cases(id),
    job_id       TEXT NOT NULL REFERENCES jobs(id),
    worker_type  TEXT NOT NULL,
    category     TEXT NOT NULL,
    confidence   FLOAT NOT NULL,
    risk_level   TEXT NOT NULL DEFAULT 'low',
    evidence     JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_findings_case  ON findings(case_id);
CREATE INDEX IF NOT EXISTS idx_findings_risk  ON findings(risk_level);
CREATE INDEX IF NOT EXISTS idx_cases_status   ON cases(status);
```

También añadir `case_id TEXT` a la tabla `jobs` existente:
```sql
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS case_id TEXT REFERENCES cases(id);
```

**Tarea A-2: Actualizar modelos Go**

Archivo: `internal/models/job.go`

Añadir las nuevas operaciones y el modelo Case:

```go
// Operaciones nuevas
const (
    OpAnalyzeText  Operation = "analyze_text"
    OpAnalyzeImage Operation = "analyze_image"
    OpAnalyzeAudio Operation = "analyze_audio"
    // Mantener las existentes para compatibilidad
)

// Nuevo modelo
type Case struct {
    ID          string     `json:"id"`
    Name        string     `json:"name"`
    Description string     `json:"description"`
    Status      string     `json:"status"`
    Priority    int        `json:"priority"`
    RiskScore   float64    `json:"risk_score"`
    CreatedAt   time.Time  `json:"created_at"`
    CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type Finding struct {
    ID         string                 `json:"id"`
    CaseID     string                 `json:"case_id"`
    JobID      string                 `json:"job_id"`
    WorkerType string                 `json:"worker_type"`
    Category   string                 `json:"category"`
    Confidence float64                `json:"confidence"`
    RiskLevel  string                 `json:"risk_level"`
    Evidence   map[string]interface{} `json:"evidence"`
    CreatedAt  time.Time              `json:"created_at"`
}
```

**Tarea A-3: Endpoints de Cases en el API**

Archivo: `internal/coordinator/api.go`

Añadir al Router():
```go
mux.HandleFunc("POST /cases", a.createCase)
mux.HandleFunc("GET /cases", a.listCases)
mux.HandleFunc("GET /cases/{id}", a.getCase)
mux.HandleFunc("GET /cases/{id}/report", a.getCaseReport)
mux.HandleFunc("POST /findings", a.submitFinding)
```

Lógica de `createCase`:
1. Parsear body con `name`, `description`, `priority`, `files[]` (array de paths)
2. Insertar el case en DB con status `queued`
3. Por cada archivo en `files[]`, detectar tipo (text/image/audio por extensión)
4. Crear un `Job` por cada archivo con `case_id` y la operación correspondiente
5. Encolar cada job en Redis
6. Retornar el case creado con sus jobs

**Tarea A-4: Actualizar Docker Compose**

Archivo: `docker-compose.yml`

Añadir los 3 workers Python:

```yaml
text-worker-1:
  build:
    context: ./workers/text_worker
    dockerfile: Dockerfile
  environment:
    WORKER_ID: text-worker-1
    COORDINATOR_URL: http://coordinator:8080
    DATABASE_URL: postgres://media:media@postgres:5432/mediaplatform?sslmode=disable
    WORKER_POOL_SIZE: "2"
  depends_on:
    - coordinator

image-worker-1:
  build:
    context: ./workers/image_worker
    dockerfile: Dockerfile
  environment:
    WORKER_ID: image-worker-1
    COORDINATOR_URL: http://coordinator:8080
    DATABASE_URL: postgres://media:media@postgres:5432/mediaplatform?sslmode=disable
    WORKER_POOL_SIZE: "2"
  depends_on:
    - coordinator

audio-worker-1:
  build:
    context: ./workers/audio_worker
    dockerfile: Dockerfile
  environment:
    WORKER_ID: audio-worker-1
    COORDINATOR_URL: http://coordinator:8080
    DATABASE_URL: postgres://media:media@postgres:5432/mediaplatform?sslmode=disable
    WORKER_POOL_SIZE: "1"
  depends_on:
    - coordinator
```

---

#### SEMANA 2 — Integración (días 6-10)

**Tarea A-5: Endpoint `submitFinding` (para que B pueda escribir resultados)**

Los workers Python escriben findings directamente vía HTTP al coordinator:

```go
func (a *API) submitFinding(w http.ResponseWriter, r *http.Request) {
    var f models.Finding
    json.NewDecoder(r.Body).Decode(&f)
    f.ID = uuid.New().String()
    f.CreatedAt = time.Now()
    // Insertar en DB
    // Actualizar risk_score del case si confidence > threshold
    // Retornar 201
}
```

**Tarea A-6: Lógica de cierre de Case**

Cuando todos los jobs de un `case_id` están en `completed` o `failed`:
- Calcular `risk_score` como promedio ponderado de los findings
- Actualizar `cases.status = 'completed'`
- Actualizar `cases.completed_at = NOW()`
- Emitir por WebSocket una notificación de case completado

Implementar como función llamada desde el handler `jobProgress` cuando detecta que todos los jobs del case terminaron.

**Tarea A-7: Endpoint del Scheduler para respetar `case_id`**

El scheduler existente funciona, pero el job que envía al worker debe incluir `case_id` en el payload. Verificar que `models.Job` incluya `CaseID` y que el `XAdd` a Redis lo serialize.

---

#### SEMANA 3 — Monitoreo y Pruebas (días 11-15)

**Tarea A-8: Actualizar WebSocket snapshot**

Archivo: `internal/coordinator/ws.go`

Añadir al `SystemSnapshot`:
```go
type SystemSnapshot struct {
    Workers    interface{}        `json:"workers"`
    Jobs       interface{}        `json:"jobs"`
    Cases      interface{}        `json:"cases"`      // NUEVO
    QueueDepth QueueDepthSnapshot `json:"queue_depth"`
    Stats      interface{}        `json:"stats"`
}
```

En `cmd/coordinator/main.go`, incluir los cases activos en el snapshot broadcast.

**Tarea A-9: Script de prueba de carga adaptado**

Adaptar `cmd/client/main.go` o crear `cmd/case_client/main.go` que:
1. Suba un directorio de archivos mixtos (txt + jpg + mp3)
2. Cree un case vía `POST /cases`
3. Monitoree hasta completar
4. Imprima el reporte final

**Tarea A-10: Documentación de despliegue**

Actualizar `docs/architecture.md` y `README.md` con:
- Nuevas variables de entorno
- Cómo levantar los workers Python
- Ejemplos de uso de la API de cases

---

### 3.2 Entregables de A

| Entregable | Archivo(s) | Semana |
|---|---|---|
| Schema DB extendido | `internal/db/db.go` | 1 |
| Modelos Go actualizados | `internal/models/job.go` | 1 |
| Endpoints /cases | `internal/coordinator/api.go` | 1 |
| Docker Compose + workers Python | `docker-compose.yml` | 1 |
| Endpoint /findings | `internal/coordinator/api.go` | 2 |
| Cierre automático de cases | `internal/coordinator/api.go` | 2 |
| WebSocket extendido | `internal/coordinator/ws.go` | 3 |
| Script de prueba de casos | `cmd/case_client/` | 3 |
| Docs actualizados | `docs/`, `README.md` | 3 |

---

## 4. INTEGRANTE B — Workers Python (Análisis ML)

> **Lenguaje:** Python 3.11 + FastAPI  
> **Dependencia de A:** Contrato de interfaz (Sección 2), Docker Compose base

### 4.1 Estructura de carpetas

```
workers/
├── shared/
│   ├── models.py          # Finding, Evidence dataclasses
│   ├── worker_base.py     # clase base: registro, heartbeat, reporting
│   └── requirements.txt   # requests, psycopg2
├── text_worker/
│   ├── main.py
│   ├── analyzer.py
│   ├── requirements.txt   # spacy, transformers, torch
│   └── Dockerfile
├── image_worker/
│   ├── main.py
│   ├── analyzer.py
│   ├── requirements.txt   # ultralytics, opencv-python
│   └── Dockerfile
└── audio_worker/
    ├── main.py
    ├── analyzer.py
    ├── requirements.txt   # openai-whisper, torch
    └── Dockerfile
```

### 4.2 Clase base `worker_base.py`

```python
import threading, requests, os, time, uuid

class WorkerBase:
    def __init__(self, worker_id, pool_size=2):
        self.worker_id = worker_id
        self.coordinator = os.getenv("COORDINATOR_URL", "http://coordinator:8080")
        self.pool_size = pool_size
        self.active_jobs = 0
        self._register()
        threading.Thread(target=self._heartbeat_loop, daemon=True).start()

    def _register(self):
        hostname = f"{self.worker_id}:8090"
        requests.post(f"{self.coordinator}/workers/register",
                      json={"id": self.worker_id, "hostname": hostname})

    def _heartbeat_loop(self):
        while True:
            try:
                requests.post(
                    f"{self.coordinator}/workers/{self.worker_id}/heartbeat",
                    json={"cpu_percent": 0, "mem_percent": 0, 
                          "active_jobs": self.active_jobs}
                )
            except:
                pass
            time.sleep(1)

    def report_progress(self, job_id, progress, status="running", error=""):
        requests.post(f"{self.coordinator}/jobs/{job_id}/progress",
                      json={"progress": progress, "status": status, "error": error})

    def submit_finding(self, finding: dict):
        requests.post(f"{self.coordinator}/findings", json=finding)
```

### 4.3 Tarea B-1: Text Worker

`analyze.py` debe ejecutar en secuencia:
1. Leer el archivo (txt o JSON de export de chat)
2. Tokenizar con spaCy (`es_core_news_sm` o `en_core_web_sm`)
3. Análisis de sentimiento con HuggingFace pipeline
4. Detección de keywords de riesgo (lista configurable)
5. Por cada hallazgo relevante (confidence > 0.5), crear un Finding y enviarlo vía `POST /findings`
6. Reportar progreso al coordinator cada 10% de procesamiento

**Stub inicial (semana 1):**
```python
def analyze(file_path: str, job_id: str, case_id: str) -> list[dict]:
    # Stub: retorna findings ficticios para que C pueda desarrollar el visor
    return [{
        "job_id": job_id, "case_id": case_id,
        "worker_type": "text", "category": "keyword",
        "confidence": 0.87, "risk_level": "high",
        "evidence": {"text_fragment": "ejemplo de texto", "keywords": ["test"]}
    }]
```

### 4.4 Tarea B-2: Image Worker

Pipeline:
1. OpenCV: cargar imagen, resize a max 640px
2. YOLOv8 (`yolov8n.pt`): inferencia de detección de objetos
3. Filtrar clases de riesgo: `knife`, `gun`, `person` (lista configurable)
4. Por cada detección con confidence > threshold, crear Finding con bounding_box
5. Reportar progreso

### 4.5 Tarea B-3: Audio Worker

Pipeline:
1. Whisper `base`: transcribir el audio a texto
2. Pasar el texto transcrito al mismo pipeline del Text Worker
3. Incluir timestamps de Whisper en la evidencia
4. Crear Findings con `transcription` y `timestamp`

### 4.6 Entregables de B

| Entregable | Archivo(s) | Semana |
|---|---|---|
| Clase base worker | `workers/shared/worker_base.py` | 1 |
| Modelo Finding | `workers/shared/models.py` | 1 |
| Stubs de los 3 workers | `*/main.py` + `*/analyzer.py` (stub) | 1 |
| Dockerfiles base | `*/Dockerfile` | 1 |
| Text Worker real | `text_worker/analyzer.py` | 2 |
| Image Worker real | `image_worker/analyzer.py` | 2 |
| Audio Worker real | `audio_worker/analyzer.py` | 2-3 |

---

## 5. INTEGRANTE C — API de Casos, Dashboard y Reportes

> **Lenguaje:** React (dashboard existente) + Python (report engine)  
> **Dependencia de A:** Endpoints `/cases`, `/findings` operativos  
> **Dependencia de B:** Schema de Finding acordado (Sección 2.4)

### 5.1 Tarea C-1: Tab "Cases" en Dashboard

Archivo: `dashboard/src/app/app.jsx`

Añadir `'Cases'` al array `TABS`. Crear componentes:

- `CaseList.jsx` — tabla de casos con estado, risk score, barra de progreso
- `CaseDetail.jsx` — vista de un caso: jobs asociados + findings agrupados por tipo
- `EvidenceViewer.jsx` — muestra el fragmento de texto resaltado, la imagen con bounding box, o el fragmento de audio transcrito

### 5.2 Tarea C-2: Report Engine

Archivo nuevo: `reports/report_engine.py`

```python
def generate_report(case_id: str, db_conn) -> dict:
    # 1. Obtener case + todos sus findings de DB
    # 2. Agrupar findings por worker_type
    # 3. Calcular risk_score final (promedio ponderado por confidence)
    # 4. Construir estructura de reporte
    return {
        "case_id": case_id,
        "generated_at": datetime.now().isoformat(),
        "risk_score": 7.3,
        "summary": {...},
        "findings_by_type": {
            "text": [...],
            "image": [...],
            "audio": [...]
        },
        "timeline": [...],   # findings ordenados por timestamp
        "recommendations": [...]
    }
```

### 5.3 Tarea C-3: Endpoint GET /cases/{id}/report

Coordinar con A para que este endpoint en Go llame al `report_engine.py` (puede ser vía subprocess o microservicio HTTP separado).

### 5.4 Tarea C-4: Notificaciones en tiempo real

El WebSocket ya existe. C debe mostrar alertas en el dashboard cuando:
- Un case cambia a `completed`
- Se detecta un finding con `risk_level = 'critical'`

Usar el snapshot de WebSocket extendido por A (tarea A-8).

### 5.5 Entregables de C

| Entregable | Archivo(s) | Semana |
|---|---|---|
| Tab Cases básico (lista) | `dashboard/src/components/CaseList.jsx` | 1-2 |
| Visor de evidencias | `dashboard/src/components/EvidenceViewer.jsx` | 2 |
| Report engine Python | `reports/report_engine.py` | 2 |
| Notificaciones WS | `dashboard/src/hooks/useSystemState.js` | 3 |
| UI final pulida | Todo el dashboard | 3 |

---

## 6. Flujo de Trabajo entre Integrantes

### 6.1 Orden de desbloqueo

```
DÍA 1-2 (A):
  └── Acordar contrato de interfaz (Sección 2) ← TODOS firman esto

DÍA 2-3 (A):
  └── DB schema + modelos Go + Docker Compose base
      ↓ desbloquea
      └── (B) puede crear Dockerfiles y stubs
      └── (C) puede desarrollar UI con datos mock

DÍA 3-5 (B):
  └── worker_base.py + stubs funcionando con coordinator
      ↓ desbloquea
      └── (A) puede probar los endpoints de cases end-to-end
      └── (C) puede ver findings reales (aunque sean stubs)

SEMANA 2 (B):
  └── Workers reales con ML
      ↓ desbloquea
      └── (C) report engine con datos reales
      └── (A) pruebas de carga con análisis real
```

### 6.2 Reglas de Git

```
main          ← producción, solo merge desde develop con aprobación de los 3
develop       ← integración, merge frecuente desde feat/*
feat/A-*      ← ramas de A
feat/B-*      ← ramas de B
feat/C-*      ← ramas de C
```

**Regla de merge:** cualquier `feat/*` → `develop` requiere 1 approval.  
`develop` → `main` requiere los 3 approvals (solo en hitos).

**Archivos de propiedad (para evitar conflictos):**

| Archivo/Directorio | Owner |
|---|---|
| `internal/` (Go) | A |
| `cmd/coordinator/`, `cmd/worker/` | A |
| `docker-compose.yml` | A |
| `workers/` (Python) | B |
| `dashboard/src/` | C |
| `reports/` | C |
| `docs/` | Todos (comentar antes de tocar) |
| `internal/models/job.go` | A (B y C proponen cambios vía PR) |

### 6.3 Protocolo de cambio de contrato

Si alguien necesita cambiar algo de la Sección 2:
1. Abrir un issue en GitHub con el cambio propuesto
2. Los otros dos deben aprobar antes de implementar
3. El cambio se hace en un commit atómico que toca todos los lados afectados

---

## 7. Semana por Semana — Vista Consolidada

### Semana 1 (días 1-5): Fundamentos

| Día | A | B | C |
|---|---|---|---|
| 1 | Acordar contrato, branch setup | Leer contrato, setup Python | Leer contrato, revisar dashboard |
| 2 | DB schema, modelos Go | worker_base.py, Finding model | CaseList.jsx con mock data |
| 3 | /cases endpoints (create, list) | Dockerfiles, stubs text+image | EvidenceViewer placeholder |
| 4 | Docker Compose + workers | Audio stub, probar con coordinator | Conectar CaseList al WS |
| 5 | /findings endpoint | Stubs integrados y funcionando | PR review + ajustes |

### Semana 2 (días 6-10): Análisis Real

| Día | A | B | C |
|---|---|---|---|
| 6 | Cierre automático de cases | Text Worker real (spaCy + HF) | Report engine básico |
| 7 | WS snapshot extendido | Image Worker real (YOLO) | Report engine avanzado |
| 8 | Case client CLI | Audio Worker (Whisper) | Endpoint /report integrado |
| 9 | Pruebas de integración | Fine-tuning modelos | Notificaciones WS |
| 10 | Fix bugs de integración | Tests de workers | UI de reportes |

### Semana 3 (días 11-15): Pulido y Entregables

| Día | A | B | C |
|---|---|---|---|
| 11 | Pruebas de carga (50+ casos) | Pruebas con datos reales | Dashboard final |
| 12 | Documentación técnica | Documentación de workers | Exportar PDF de reporte |
| 13 | README de despliegue | Performance tuning | Tests E2E de UI |
| 14 | Entregable 2: código | Entregable 2: código | Entregable 2: código |
| 15 | Entregable 3: reporte final | Entregable 3: métricas ML | Entregable 3: casos de estudio |

---

## 8. Variables de Entorno — Referencia Completa

### Coordinator (Go)

| Variable | Default | Descripción |
|---|---|---|
| `DATABASE_URL` | postgres://... | Conexión PostgreSQL |
| `REDIS_ADDR` | redis:6379 | Broker Redis |
| `PORT` | 8080 | Puerto HTTP |
| `DATASET_PATH` | /app/dataset/files | Ruta de archivos analizables |

### Workers Python

| Variable | Default | Descripción |
|---|---|---|
| `WORKER_ID` | text-worker-1 | ID único del worker |
| `COORDINATOR_URL` | http://coordinator:8080 | URL del coordinator |
| `DATABASE_URL` | postgres://... | Para escribir findings directamente |
| `WORKER_POOL_SIZE` | 2 | Threads de procesamiento paralelo |
| `MODEL_CONFIDENCE_THRESHOLD` | 0.5 | Mínimo confidence para reportar |

---

## 9. Criterios de Evaluación — Mapeo con Rúbrica

| Criterio (rúbrica) | % | Responsable principal | Cómo se evidencia |
|---|---|---|---|
| Arquitectura Distribuida | 20% | A | Coordinator + 3 workers en Docker, Redis Streams |
| Gestión de Procesos y Concurrencia | 15% | A + B | Goroutine pool (Go), thread pool (Python) |
| Gestión de Cargas de Trabajo | 10% | A | Prioridades Redis, estados de case/job |
| Procesamiento Multimedia | 10% | B | 3 workers con ML real funcionando |
| Dashboard y Monitoreo | 10% | C | Dashboard con cases, workers, findings en tiempo real |
| Reportes y Evidencias | 10% | C | Report engine, visor de evidencias |
| Escalabilidad y Rendimiento | 10% | A | Pruebas con 50+ casos, Grafana metrics |
| Seguridad y Acceso | 5% | A | Auth básica o validación de inputs |
| Documentación Técnica | 5% | Todos | README, arquitectura, instrucciones |
| Repositorio y Buenas Prácticas | 5% | Todos | Git flow, commits atómicos, estructura |

---

## 10. Checklist de Entregables

### Entregable 1: Documentación de diseño
- [ ] Diagrama de arquitectura actualizado
- [ ] Modelo de datos (tablas cases, jobs, findings)
- [ ] Diagrama de flujo del análisis de un caso
- [ ] Especificación de APIs

### Entregable 2: Código fuente + instrucciones
- [ ] `make up` levanta todo el sistema
- [ ] `POST /cases` crea un caso y encola jobs
- [ ] Workers Python analizan y reportan findings
- [ ] Dashboard muestra cases, jobs, findings en tiempo real
- [ ] Report engine genera reporte consolidado

### Entregable 3: Reporte final
- [ ] Resultados de prueba con 50+ casos
- [ ] Métricas de rendimiento (throughput, latencia)
- [ ] Análisis de escalabilidad (¿qué pasa con 3 workers vs 6?)
- [ ] Caso de estudio documentado
