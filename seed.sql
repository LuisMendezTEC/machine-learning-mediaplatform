-- 1. Crear el caso
INSERT INTO cases (id, name, description, status, priority, risk_score) 
VALUES ('caso-prueba-123', 'Análisis de Interceptación', 'Caso inyectado', 'completed', 5, 8.5) 
ON CONFLICT (id) DO NOTHING;

-- 2. Crear el trabajo (agregando file_id)
INSERT INTO jobs (id, case_id, file_id, file_path, operation, status, priority, progress) 
VALUES ('job-falso-456', 'caso-prueba-123', 'file-falso-789', '/app/dataset/files/audio_falso.mp3', 'extract_audio', 'completed', 5, 100) 
ON CONFLICT (id) DO NOTHING;

-- 3. Crear el hallazgo
INSERT INTO findings (id, case_id, job_id, worker_type, category, confidence, risk_level, evidence) 
VALUES ('hallazgo-1', 'caso-prueba-123', 'job-falso-456', 'audio', 'Detección de Amenaza', 0.92, 'critical', '{"timestamp": "00:01:23", "transcription": "El paquete está en la ubicación acordada."}') 
ON CONFLICT (id) DO NOTHING;