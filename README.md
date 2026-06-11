# Documentación de Diseño del Sistema
## Análisis Multiproceso y Distribuido de Datos en Aplicaciones de Mensajería v2.0

**Proyecto base:** `mediaplatform` (Go)  
**Fecha:** Junio 2026  
**Equipo:** 3 integrantes — Infraestructura (A), Workers ML (B), API/Dashboard/Reportes (C)

---

## 1. Descripción General

El sistema es una plataforma distribuida que recibe **casos de análisis forense digital** compuestos por archivos de mensajería (textos exportados, imágenes, audios) y los procesa de forma **concurrente y asíncrona** mediante workers especializados. El resultado es un **reporte consolidado** con hallazgos clasificados por tipo, nivel de riesgo y evidencia asociada.

---

## 2. Arquitectura del Sistema

> Ver imagen adjunta: `architecture_diagram.svg`

### 2.1 Componentes Principales

| Componente | Tecnología | Responsabilidad |
|---|---|---|
| Coordinator | Go 1.26 | Orquestación, API REST, WebSocket, Scheduler |
| Text Worker | Python 3.11 + FastAPI | Análisis NLP de texto exportado |
| Image Worker | Python 3.11 + FastAPI | Detección de objetos en imágenes |
| Audio Worker | Python 3.11 + FastAPI | Transcripción y análisis de audio |
| Redis Streams | Redis 7 | Cola de prioridades con consumer groups |
| PostgreSQL | PostgreSQL 16 | Persistencia de estado e histórico |
| MinIO | MinIO latest | Almacenamiento de archivos resultado |
| Prometheus + Grafana | Latest | Métricas y observabilidad |
| Dashboard | React 18 + Vite | Interfaz de monitoreo en tiempo real |

### 2.2 Principios de Diseño

- **Desacoplamiento total**: el Coordinator no conoce la implementación interna de los workers. El contrato es HTTP.
- **Priorización sin starvation**: Redis Streams con 3 colas (`jobs:high`, `jobs:normal`, `jobs:low`). Un proceso de Celery Beat promueve tareas envejecidas.
- **Fault tolerance**: workers que dejan de enviar heartbeat son eviccionados y sus jobs re-encolados automáticamente en 15 segundos.
- **Escalabilidad horizontal**: agregar workers es solo añadir un contenedor al `docker-compose.yml`.

---

## 3. Modelos de Datos

### 3.1 Tabla `cases`

Representa un **caso de análisis** que agrupa múltiples archivos de una misma investigación.

```sql
CREATE TABLE cases (
    id           TEXT PRIMARY KEY,           -- UUID v4
    name         TEXT NOT NULL,              -- Nombre descriptivo del caso
    description  TEXT,                       -- Descripción opcional
    status       TEXT NOT NULL DEFAULT 'queued',
                 -- queued | processing | completed | failed
    priority     INT  NOT NULL DEFAULT 5,    -- 1 (baja) a 10 (crítica)
    risk_score   FLOAT DEFAULT 0,            -- 0.0 a 10.0 calculado al cierre
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ                 -- NULL hasta cierre
);
```

**Estados del case:**

```
queued ──► processing ──► completed
                    └────► failed
```

---

### 3.2 Tabla `jobs`

Representa **una tarea de análisis** sobre un archivo específico dentro de un case.

```sql
CREATE TABLE jobs (
    id           TEXT PRIMARY KEY,           -- UUID v4
    case_id      TEXT REFERENCES cases(id),  -- ← NUEVO
    file_id      TEXT NOT NULL,
    file_path    TEXT NOT NULL,              -- Ruta al archivo en el sistema
    operation    TEXT NOT NULL,
                 -- analyze_text | analyze_image | analyze_audio
                 -- (+ convert, extract_audio, thumbnail del proyecto base)
    status       TEXT NOT NULL DEFAULT 'pending',
                 -- pending | assigned | running | completed | failed
    priority     INT  NOT NULL DEFAULT 5,
    worker_id    TEXT,                       -- Worker asignado
    progress     INT  NOT NULL DEFAULT 0,    -- 0-100
    error_msg    TEXT,
    result_url   TEXT,                       -- URL en MinIO
    retries      INT  NOT NULL DEFAULT 0,
    max_retries  INT  NOT NULL DEFAULT 3,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at   TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);
```

**Estados del job:**

```
pending ──► assigned ──► running ──► completed
                   └─────────────► failed ──► pending (retry)
```

---

### 3.3 Tabla `findings`

Almacena **cada hallazgo detectado** por un worker durante el análisis.

```sql
CREATE TABLE findings (
    id           TEXT PRIMARY KEY,           -- UUID v4
    case_id      TEXT NOT NULL REFERENCES cases(id),
    job_id       TEXT NOT NULL REFERENCES jobs(id),
    worker_type  TEXT NOT NULL,              -- text | image | audio
    category     TEXT NOT NULL,
                 -- keyword | sentiment | offensive | threat |
                 -- violence | weapon | drug | explicit | suspicious
    confidence   FLOAT NOT NULL,            -- 0.0 a 1.0
    risk_level   TEXT NOT NULL DEFAULT 'low',
                 -- low | medium | high | critical
    evidence     JSONB,                     -- Ver 3.4
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_findings_case      ON findings(case_id);
CREATE INDEX idx_findings_risk      ON findings(risk_level);
CREATE INDEX idx_findings_category  ON findings(category);
```

---

### 3.4 Estructura del campo `evidence` (JSONB)

El campo `evidence` varía según el tipo de worker:

**Text Worker:**
```json
{
  "text_fragment": "fragmento exacto del mensaje detectado",
  "message_index": 42,
  "timestamp": "2024-01-15T10:23:00Z",
  "keywords": ["keyword1", "keyword2"],
  "sentiment_score": -0.87,
  "language": "es"
}
```

**Image Worker:**
```json
{
  "detected_class": "knife",
  "confidence": 0.91,
  "bounding_box": { "x": 120, "y": 80, "width": 200, "height": 150 },
  "image_path": "/app/dataset/files/imagen.jpg",
  "frame_index": null
}
```

**Audio Worker:**
```json
{
  "transcription": "texto transcrito del fragmento de audio",
  "timestamp_start": "00:01:23",
  "timestamp_end": "00:01:45",
  "keywords": ["keyword1"],
  "audio_path": "/app/dataset/files/audio.mp3",
  "language": "es",
  "confidence_stt": 0.93
}
```

---

### 3.5 Tabla `worker_registry`

Registro persistente de workers para sobrevivir reinicios del coordinator.

```sql
CREATE TABLE worker_registry (
    id        TEXT PRIMARY KEY,
    hostname  TEXT NOT NULL,           -- "text-worker-1:8090"
    status    TEXT NOT NULL DEFAULT 'idle',
    last_seen TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

### 3.6 Índices adicionales

```sql
CREATE INDEX idx_jobs_status    ON jobs(status);
CREATE INDEX idx_jobs_worker    ON jobs(worker_id);
CREATE INDEX idx_jobs_priority  ON jobs(priority DESC);
CREATE INDEX idx_jobs_case      ON jobs(case_id);
CREATE INDEX idx_cases_status   ON cases(status);
```

---

### 3.7 Diagrama Entidad-Relación

```
┌─────────────────┐          ┌──────────────────────┐
│     CASES       │ 1      N │        JOBS           │
├─────────────────┤──────────┤──────────────────────┤
│ PK id           │          │ PK id                │
│    name         │          │ FK case_id           │
│    description  │          │    file_path         │
│    status       │          │    operation         │
│    priority     │          │    status            │
│    risk_score   │          │    worker_id         │
│    created_at   │          │    progress          │
│    completed_at │          │    retries           │
└─────────────────┘          │    created_at        │
                             │    started_at        │
                             │    completed_at      │
                             └──────────────────────┘
                                          │ 1
                                          │
                                          │ N
                             ┌──────────────────────┐
                             │      FINDINGS         │
                             ├──────────────────────┤
                             │ PK id                │
                             │ FK case_id           │
                             │ FK job_id            │
                             │    worker_type       │
                             │    category          │
                             │    confidence        │
                             │    risk_level        │
                             │    evidence (JSONB)  │
                             │    created_at        │
                             └──────────────────────┘
```

---

## 4. Especificación de la API REST

### 4.1 Endpoints de Cases

#### `POST /cases` — Crear caso de análisis
```json
Request:
{
  "name": "Investigación Usuario X",
  "description": "Análisis de exportación de WhatsApp",
  "priority": 8,
  "files": [
    "/app/dataset/files/chat_export.txt",
    "/app/dataset/files/imagen_enviada.jpg",
    "/app/dataset/files/nota_de_voz.mp3"
  ]
}

Response 201:
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Investigación Usuario X",
  "status": "queued",
  "priority": 8,
  "jobs_created": 3,
  "created_at": "2026-01-15T10:00:00Z"
}
```

#### `GET /cases` — Listar todos los casos
```
Response 200: [ { case_object }, ... ]
```

#### `GET /cases/{id}` — Detalle de un caso
```json
Response 200:
{
  "id": "...",
  "name": "...",
  "status": "processing",
  "risk_score": 6.4,
  "jobs": [
    { "id": "...", "operation": "analyze_text", "status": "completed" },
    { "id": "...", "operation": "analyze_image", "status": "running", "progress": 60 },
    { "id": "...", "operation": "analyze_audio", "status": "pending" }
  ],
  "findings_count": 12
}
```

#### `GET /cases/{id}/report` — Reporte consolidado
```json
Response 200:
{
  "case_id": "...",
  "generated_at": "2026-01-15T10:15:00Z",
  "risk_score": 7.8,
  "risk_level": "high",
  "summary": {
    "total_findings": 15,
    "by_risk": { "critical": 2, "high": 5, "medium": 6, "low": 2 },
    "by_type": { "text": 8, "image": 4, "audio": 3 }
  },
  "findings": [
    {
      "id": "...",
      "worker_type": "text",
      "category": "threat",
      "confidence": 0.93,
      "risk_level": "critical",
      "evidence": { "text_fragment": "...", "keywords": ["amenaza"] }
    }
  ],
  "timeline": [ ... ]
}
```

---

### 4.2 Endpoints de Workers (internos)

| Endpoint | Método | Descripción |
|---|---|---|
| `/workers/register` | POST | Worker se registra al arrancar |
| `/workers/{id}/heartbeat` | POST | Heartbeat cada 1 segundo |
| `/workers` | GET | Lista todos los workers y su estado |
| `/tasks` | POST | Coordinator envía job al worker |
| `/jobs/{id}/progress` | POST | Worker reporta progreso y estado |
| `/findings` | POST | Worker reporta un hallazgo detectado |

---

## 5. Contrato Worker ↔ Coordinator

### 5.1 Registro de worker

El worker envía al arrancar:
```json
POST /workers/register
{ "id": "text-worker-1", "hostname": "text-worker-1:8090" }
```

### 5.2 Heartbeat (cada 1 segundo)

```json
POST /workers/{id}/heartbeat
{ "cpu_percent": 45.2, "mem_percent": 62.1, "active_jobs": 2 }
```

### 5.3 Recepción de job

El Coordinator envía al worker:
```json
POST /tasks  (al hostname del worker)
{
  "id": "<job_uuid>",
  "case_id": "<case_uuid>",
  "file_path": "/app/dataset/files/archivo.txt",
  "operation": "analyze_text",
  "priority": 8
}
```

El worker responde:
- `202 Accepted` — job aceptado en el pool
- `429 Too Many Requests` — pool lleno, el coordinator re-encola

### 5.4 Envío de finding

El worker envía cada hallazgo encontrado:
```json
POST /findings
{
  "job_id": "<uuid>",
  "case_id": "<uuid>",
  "worker_type": "text",
  "category": "threat",
  "confidence": 0.93,
  "risk_level": "critical",
  "evidence": {
    "text_fragment": "...",
    "keywords": ["amenaza", "violencia"],
    "timestamp": "2024-01-15T09:30:00Z"
  }
}
```

---

## 6. Estructura de Directorios del Proyecto

```
mediaplatform/
├── cmd/
│   ├── coordinator/main.go      # Punto de entrada del coordinator
│   ├── worker/main.go           # Worker Go legado (FFmpeg)
│   └── case_client/main.go      # CLI para crear casos y cargas de prueba
│
├── internal/
│   ├── coordinator/
│   │   ├── api.go               # Handlers HTTP: /cases, /findings, /ws, etc.
│   │   ├── registry.go          # Registro y heartbeat de workers
│   │   ├── scheduler.go         # Dequeue, asignación least-loaded
│   │   └── ws.go                # WebSocket hub y broadcast
│   ├── db/db.go                 # Schema SQL, migraciones, helpers CRUD
│   ├── models/job.go            # Structs: Job, Case, Finding, WorkerInfo
│   ├── queue/queue.go           # Redis Streams: Enqueue/Dequeue/Ack
│   ├── multimedia/              # FFmpeg wrappers (heredado)
│   ├── monitoring/metrics.go    # Prometheus metrics
│   └── storage/minio.go         # Upload a MinIO
│
├── workers/
│   ├── shared/
│   │   ├── worker_base.py       # Clase base: registro, heartbeat, reporting
│   │   └── models.py            # Finding, Evidence dataclasses
│   ├── text_worker/
│   │   ├── main.py              # FastAPI app + pool de threads
│   │   ├── analyzer.py          # spaCy + HuggingFace pipeline
│   │   ├── requirements.txt
│   │   └── Dockerfile
│   ├── image_worker/
│   │   ├── main.py
│   │   ├── analyzer.py          # YOLOv8 + OpenCV
│   │   ├── requirements.txt
│   │   └── Dockerfile
│   └── audio_worker/
│       ├── main.py
│       ├── analyzer.py          # Whisper + NLP
│       ├── requirements.txt
│       └── Dockerfile
│
├── reports/
│   └── report_engine.py         # Consolidación de findings → JSON/PDF
│
├── dashboard/
│   └── src/
│       ├── app/                 # App principal React
│       ├── components/
│       │   ├── CaseList.jsx     # Lista de casos con estado y risk score
│       │   ├── EvidenceViewer.jsx # Visor de evidencias (texto/imagen/audio)
│       │   └── ...              # Componentes existentes
│       └── hooks/
│           └── useSystemState.js # WebSocket hook (extendido con cases)
│
├── docker-compose.yml           # Todos los servicios
├── Dockerfile.coordinator
├── Dockerfile.worker            # Worker Go legado
├── Makefile
└── docs/
    ├── architecture.md
    └── DISEÑO_SISTEMA.md        # Este documento
```

---

## 7. Flujo de Análisis de un Caso

> Ver imagen adjunta: `flow_diagram.svg`

### Paso a paso

| # | Actor | Acción |
|---|---|---|
| 1 | Cliente | `POST /cases` con nombre, descripción, archivos y prioridad |
| 2 | Coordinator | INSERT case en PostgreSQL (status: `queued`) |
| 3 | Coordinator | Por cada archivo: detectar tipo → crear Job con `operation` correspondiente |
| 4 | Coordinator | XADD cada job a Redis (`jobs:high`, `jobs:normal` o `jobs:low` según prioridad) |
| 5 | Scheduler | XReadGroup (dequeue) → seleccionar worker con menos jobs activos (least-loaded) |
| 6 | Coordinator | POST /tasks al hostname del worker seleccionado |
| 7 | Worker | Recibir job → UPDATE status `assigned` → `running` |
| 8 | Worker | Ejecutar pipeline ML (spaCy / YOLOv8 / Whisper) |
| 9 | Worker | Por cada hallazgo: POST /findings + INSERT en tabla `findings` |
| 10 | Worker | POST /jobs/{id}/progress con status `completed` |
| 11 | Coordinator | Verificar: ¿todos los jobs del case están en `completed` o `failed`? |
| 12 | Coordinator | Calcular `risk_score` final = promedio ponderado por confidence |
| 13 | Coordinator | UPDATE cases SET status=`completed`, risk_score=N |
| 14 | Coordinator | WebSocket broadcast → Dashboard actualiza UI |
| 15 | Cliente | `GET /cases/{id}/report` |
| 16 | Report Engine | SELECT findings WHERE case_id=… → consolidar por tipo → JSON |
| 17 | Coordinator | Retornar reporte con findings, timeline y score |

---

## 8. Sistema de Prioridades y Anti-Starvation

### Niveles de prioridad

| Nivel | Rango | Stream Redis | Descripción |
|---|---|---|---|
| CRITICAL | 9-10 | `jobs:high` | Casos urgentes, procesamiento inmediato |
| ALTA | 8 | `jobs:high` | Prioridad alta |
| MEDIA | 4-7 | `jobs:normal` | Flujo estándar |
| BAJA | 1-3 | `jobs:low` | Procesamiento diferido |

### Mecanismo Anti-Starvation (Priority Aging)

Un proceso periódico (Celery Beat / ticker en Go) revisa cada 60 segundos los jobs que llevan más de N minutos en estado `pending` y los re-encola en el stream de mayor prioridad:

```
Si job lleva > 5 min en jobs:low  → mover a jobs:normal
Si job lleva > 3 min en jobs:normal → mover a jobs:high
```

Esto garantiza **fairness** y elimina **starvation** en condiciones de alta carga.

---

## 9. Monitoreo y Observabilidad

### Métricas Prometheus exportadas por cada worker

| Métrica | Tipo | Descripción |
|---|---|---|
| `worker_cpu_percent` | Gauge | CPU del proceso worker (0-100) |
| `worker_memory_mb` | Gauge | Memoria RSS en MB |
| `worker_active_jobs` | Gauge | Jobs procesándose actualmente |
| `worker_jobs_completed_total` | Counter | Total jobs completados (por operación) |
| `worker_jobs_failed_total` | Counter | Total jobs fallidos |
| `worker_job_duration_seconds` | Histogram | Tiempo de procesamiento ML |

### Dashboard WebSocket — Payload del snapshot

```json
{
  "workers": [ { "id": "...", "status": "busy", "cpu_percent": 45, "active_jobs": 2 } ],
  "jobs": [ { "id": "...", "status": "running", "progress": 65 } ],
  "cases": [ { "id": "...", "status": "processing", "risk_score": 0 } ],
  "queue_depth": { "high": 3, "normal": 12, "low": 8 },
  "stats": { "pending": 15, "running": 6, "completed": 234, "failed": 2 }
}
```

---

## 10. Instrucciones de Despliegue

### Prerrequisitos
- Docker Desktop en ejecución
- Go 1.26+ (para compilar el coordinator y cliente)
- Python 3.11+ en los contenedores (via Docker, no requerido local)

### Levantar el sistema completo

```bash
git clone <repo>
cd mediaplatform
make hooks    # configurar git hooks
make up       # construye imágenes y levanta todos los servicios
make logs     # ver logs en tiempo real
```

### Acceso a servicios

| Servicio | URL | Credenciales |
|---|---|---|
| Dashboard | http://localhost:5173 | — |
| Coordinator API | http://localhost:8080 | — |
| MinIO Console | http://localhost:9001 | minioadmin / minioadmin |
| Prometheus | http://localhost:9090 | — |
| Grafana | http://localhost:3001 | admin / admin |

### Crear un caso de prueba

```bash
# Crear un caso con archivos mixtos
curl -X POST http://localhost:8080/cases \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Caso de Prueba",
    "priority": 8,
    "files": [
      "/app/dataset/files/chat_export.txt",
      "/app/dataset/files/imagen.jpg",
      "/app/dataset/files/audio.mp3"
    ]
  }'

# Prueba de carga: 50 casos concurrentes
go run ./cmd/case_client -batch -concurrency 50 -watch
```