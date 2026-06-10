FROM python:3.11-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    libglib2.0-0 \
    libgl1 \
    libxcb1 \
    libx11-xcb1 \
    && rm -rf /var/lib/apt/lists/*

COPY workers/shared/requirements.txt /app/workers/shared/requirements.txt
COPY workers/image_worker/requirements.txt /app/workers/image_worker/requirements.txt

RUN pip install --no-cache-dir -r /app/workers/shared/requirements.txt
RUN pip install --no-cache-dir -r /app/workers/image_worker/requirements.txt

# Pre-download YOLOv8 model during build (non-fatal if network fails)
RUN python -c "from ultralytics import YOLO; YOLO('yolov8n.pt')" || echo "Warning: YOLOv8 model download failed"

COPY workers/shared /app/workers/shared
COPY workers/image_worker /app/workers/image_worker

# Pre-download specialized threat detection model during build
RUN python -c "import requests; r = requests.get('https://huggingface.co/Subh775/Threat-Detection-YOLOv8n/resolve/main/weights/best.pt'); r.raise_for_status(); open('/app/workers/image_worker/threats_yolov8n.pt', 'wb').write(r.content)" || echo "Warning: Threat model download failed"

ENV PYTHONPATH=/app
EXPOSE 8090

CMD ["python", "workers/image_worker/main.py"]

