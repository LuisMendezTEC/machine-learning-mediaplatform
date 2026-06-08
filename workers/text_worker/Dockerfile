FROM python:3.11-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements first to leverage Docker cache
COPY workers/shared/requirements.txt /app/workers/shared/requirements.txt
COPY workers/text_worker/requirements.txt /app/workers/text_worker/requirements.txt

RUN pip install --no-cache-dir -r /app/workers/shared/requirements.txt
RUN pip install --no-cache-dir -r /app/workers/text_worker/requirements.txt

# Pre-download spaCy models (non-fatal if network fails)
RUN python -m spacy download es_core_news_sm || echo "Warning: es_core_news_sm download failed"
RUN python -m spacy download en_core_web_sm || echo "Warning: en_core_web_sm download failed"

# Pre-download transformers model for local execution (non-fatal if network fails)
RUN python -c "from transformers import pipeline; pipeline('sentiment-analysis', model='lxyuan/distilbert-base-multilingual-cased-sentiments-student')" || echo "Warning: sentiment model download failed"

# Copy source code
COPY workers/shared /app/workers/shared
COPY workers/text_worker /app/workers/text_worker

ENV PYTHONPATH=/app
EXPOSE 8090

CMD ["python", "workers/text_worker/main.py"]
