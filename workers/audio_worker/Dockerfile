FROM python:3.11-slim

WORKDIR /app

# Install ffmpeg (Whisper requirement) and curl
RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    curl \
    && rm -rf /var/lib/apt/lists/*

COPY workers/shared/requirements.txt /app/workers/shared/requirements.txt
COPY workers/audio_worker/requirements.txt /app/workers/audio_worker/requirements.txt

RUN pip install --no-cache-dir -r /app/workers/shared/requirements.txt
RUN pip install --no-cache-dir -r /app/workers/audio_worker/requirements.txt

# Pre-download Whisper model during build (non-fatal if network fails)
RUN python -c "import whisper; whisper.load_model('base')" || echo "Warning: Whisper model download failed"

# Pre-download transformers model during build (non-fatal if network fails)
RUN python -c "from transformers import pipeline; pipeline('sentiment-analysis', model='lxyuan/distilbert-base-multilingual-cased-sentiments-student')" || echo "Warning: sentiment model download failed"

COPY workers/shared /app/workers/shared
COPY workers/audio_worker /app/workers/audio_worker

ENV PYTHONPATH=/app
EXPOSE 8090

CMD ["python", "workers/audio_worker/main.py"]
