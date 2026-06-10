import os
import time
import logging
import threading
import requests
import psutil
from fastapi import FastAPI, BackgroundTasks, Response, status
from pydantic import BaseModel

logging.basicConfig(level=logging.INFO, format='%(asctime)s [%(levelname)s] %(name)s: %(message)s')
logger = logging.getLogger("worker_base")

class TaskPayload(BaseModel):
    id: str
    file_path: str
    operation: str
    priority: int = 5
    case_id: str

class WorkerBase:
    def __init__(self, worker_id: str, pool_size: int = 2):
        self.worker_id = worker_id
        self.pool_size = pool_size
        self.coordinator_url = os.getenv("COORDINATOR_URL", "http://coordinator:8080")
        self.active_jobs = 0
        self._lock = threading.Lock()
        
        self.app = FastAPI(title=f"Worker {self.worker_id}")
        self.setup_routes()

    def setup_routes(self):
        @self.app.post("/tasks", status_code=202)
        def assign_task(payload: TaskPayload, background_tasks: BackgroundTasks, response: Response):
            with self._lock:
                if self.active_jobs >= self.pool_size:
                    response.status_code = status.HTTP_429_TOO_MANY_REQUESTS
                    return {"error": "worker pool full"}
                self.active_jobs += 1
            
            logger.info(f"Accepted job {payload.id} ({payload.operation}) for case {payload.case_id}")
            background_tasks.add_task(self._run_job_wrapper, payload)
            return {"status": "accepted"}

        @self.app.get("/health")
        def health():
            return {
                "status": "ok",
                "worker_id": self.worker_id,
                "active_jobs": self.active_jobs,
                "pool_size": self.pool_size
            }

    def start(self):
        # Register worker in background after a tiny delay to allow uvicorn to start
        threading.Thread(target=self._delayed_registration, daemon=True).start()
        # Start heartbeat loop
        threading.Thread(target=self._heartbeat_loop, daemon=True).start()

    def _delayed_registration(self):
        time.sleep(1) # wait for server to start up
        self._register()

    def _register(self):
        url = f"{self.coordinator_url}/workers/register"
        hostname = f"{self.worker_id}:8090"
        payload = {"id": self.worker_id, "hostname": hostname}
        logger.info(f"Registering worker at {url}...")
        for attempt in range(20):
            try:
                res = requests.post(url, json=payload, timeout=5)
                if res.status_code in (200, 201):
                    logger.info(f"Successfully registered worker {self.worker_id} at {hostname}")
                    return
                logger.warning(f"Registration returned status {res.status_code}, retrying in 3s...")
            except Exception as e:
                logger.warning(f"Registration failed: {e}, retrying in 3s...")
            time.sleep(3)
        logger.error("Failed to register worker with coordinator after 20 attempts")

    def _heartbeat_loop(self):
        url = f"{self.coordinator_url}/workers/{self.worker_id}/heartbeat"
        try:
            p = psutil.Process()
            p.cpu_percent() # Seed
        except Exception:
            p = None
        while True:
            try:
                if p:
                    cpu = p.cpu_percent() / psutil.cpu_count()
                    mem = p.memory_percent()
                else:
                    cpu = 0.0
                    mem = 0.0
                payload = {
                    "cpu_percent": float(cpu),
                    "mem_percent": float(mem),
                    "active_jobs": self.active_jobs
                }
                requests.post(url, json=payload, timeout=2)
            except Exception:
                pass
            time.sleep(1)

    def _run_job_wrapper(self, payload: TaskPayload):
        job_id = payload.id
        case_id = payload.case_id
        file_path = payload.file_path
        operation = payload.operation
        
        logger.info(f"Starting job {job_id} processing")
        # Report running status
        self.report_progress(job_id, 0, "running")
        
        try:
            self.execute_task(job_id, case_id, file_path, operation)
            logger.info(f"Job {job_id} completed successfully")
            self.report_complete(job_id, "")
        except Exception as e:
            logger.error(f"Job {job_id} failed: {e}", exc_info=True)
            self.report_fail(job_id, str(e))
        finally:
            with self._lock:
                self.active_jobs = max(0, self.active_jobs - 1)

    def execute_task(self, job_id: str, case_id: str, file_path: str, operation: str):
        raise NotImplementedError("Subclasses must implement execute_task")

    def report_progress(self, job_id: str, progress: int, status: str = "running", error: str = ""):
        url = f"{self.coordinator_url}/jobs/{job_id}/progress"
        payload = {"progress": progress, "status": status, "error": error}
        try:
            requests.post(url, json=payload, timeout=3)
        except Exception as e:
            logger.error(f"Failed to report progress for job {job_id}: {e}")

    def report_complete(self, job_id: str, result_url: str = ""):
        url = f"{self.coordinator_url}/jobs/{job_id}/complete"
        payload = {"result_url": result_url}
        try:
            requests.post(url, json=payload, timeout=3)
        except Exception as e:
            logger.error(f"Failed to report complete for job {job_id}: {e}")

    def report_fail(self, job_id: str, error_msg: str):
        url = f"{self.coordinator_url}/jobs/{job_id}/fail"
        payload = {"error_msg": error_msg}
        try:
            requests.post(url, json=payload, timeout=3)
        except Exception as e:
            logger.error(f"Failed to report failure for job {job_id}: {e}")

    def submit_finding(self, finding: dict):
        url = f"{self.coordinator_url}/findings"
        try:
            res = requests.post(url, json=finding, timeout=5)
            if res.status_code != 201:
                logger.error(f"Failed to submit finding: status {res.status_code}, response: {res.text}")
            else:
                logger.info(f"Submitted finding: {finding.get('category')} (confidence: {finding.get('confidence')})")
        except Exception as e:
            logger.error(f"Failed to submit finding: {e}")
