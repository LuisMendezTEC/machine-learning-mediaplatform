import os
import uvicorn
from workers.shared.worker_base import WorkerBase
from workers.audio_worker.analyzer import analyze_audio

class AudioWorker(WorkerBase):
    def __init__(self):
        worker_id = os.getenv("WORKER_ID", "audio-worker-1")
        pool_size = int(os.getenv("WORKER_POOL_SIZE", "1"))
        super().__init__(worker_id, pool_size)
        
    def execute_task(self, job_id: str, case_id: str, file_path: str, operation: str):
        analyze_audio(
            file_path=file_path,
            job_id=job_id,
            case_id=case_id,
            submit_finding_fn=self.submit_finding,
            report_progress_fn=self.report_progress
        )

if __name__ == "__main__":
    worker = AudioWorker()
    worker.start()
    uvicorn.run(worker.app, host="0.0.0.0", port=8090)
