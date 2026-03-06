"""
gRPC Embedding Service Server
Models used:
- BAAI/bge-small-en-v1.5 for dense embeddings
- prithivida/Splade_PP_en_v1 for sparse embeddings
"""

import grpc
from concurrent import futures
import numpy as np
import logging
from typing import List, Tuple
import os
import time
from dotenv import load_dotenv
from concurrent.futures import ThreadPoolExecutor

from fastembed import TextEmbedding, SparseTextEmbedding

import embeddingService_pb2
import embeddingService_pb2_grpc

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


def get_optimal_thread_config() -> Tuple[int, int, int]:
    """
    Dynamically calculates optimal thread configuration based on actual available hardware.
    Returns: (model_threads, executor_workers, grpc_workers)
    """
    # 1. Reliably get CPU count, respecting Docker/Linux cgroup limits if applicable
    try:
        # sched_getaffinity is Linux-specific but highly accurate for containers
        total_cores = len(os.sched_getaffinity(0))
    except AttributeError:
        # Fallback for Windows/macOS
        total_cores = os.cpu_count() or 4

    # 2. Calculate the sweet spot
    if total_cores <= 4:
        # Low resource environment: Prevent contention entirely
        model_threads = 1
        executor_workers = max(2, total_cores)
        grpc_workers = executor_workers * 2
    else:
        # High resource environment: Give models enough threads for AVX vectorization (usually maxes out usefulness at 4-8), 
        # use the rest to maximize concurrent request handling.
        model_threads = min(4, max(2, total_cores // 4))
        
        # We divide remaining cores by model_threads to see how many simultaneous 
        # model operations we can safely run without OS-level context switching
        executor_workers = max(4, total_cores // model_threads)
        
        # gRPC workers handle I/O, so we can afford slightly more of them than pure compute workers
        grpc_workers = executor_workers * 2
        
    logger.info(f"Hardware detected: {total_cores} cores available.")
    logger.info(f"Configured Models to use {model_threads} threads each.")
    logger.info(f"Configured Internal ThreadPool to {executor_workers} workers.")
    
    return model_threads, executor_workers, grpc_workers


class EmbeddingServiceServicer(embeddingService_pb2_grpc.EmbeddingServiceServicer):

    def __init__(self, model_threads: int, executor_workers: int):
        logger.info("Initializing embedding models...")
        
        self.dense_model = TextEmbedding(
            model_name="BAAI/bge-small-en-v1.5",
            max_length=512,
            threads=model_threads 
        )
        
        self.sparse_model = SparseTextEmbedding(
            model_name="prithivida/Splade_PP_en_v1",
            threads=model_threads 
        )
        
        self.executor = ThreadPoolExecutor(max_workers=executor_workers)
    
    def _get_dense_embedding(self, query: str) -> np.ndarray:
        try:
            prefix = "_Query_"
            bge_instruction = "Represent this sentence for searching relevant passages: "
            
            if query.startswith(prefix):
                logger.info("Query prefix detected, prepending BGE instruction")
                query = bge_instruction + query[len(prefix):]
            
            embedding = list(self.dense_model.embed([query]))[0]
            return embedding.astype(np.float32)
        except Exception as e:
            logger.error(f"Error generating dense embedding: {e}")
            raise
    
    def _get_dense_embeddings_batch(self, queries: List[str]) -> List[np.ndarray]:
        try:
            processed_queries = []
            bge_instruction = "Represent this sentence for searching relevant passages: "
            prefix = "_Query_"
            
            for q in queries:
                if q.startswith(prefix):
                    trimmed_q = q[len(prefix):]
                    processed_queries.append(bge_instruction + trimmed_q)
                else:
                    processed_queries.append(q)

            embeddings = list(self.dense_model.embed(processed_queries))
            return [emb.astype(np.float32) for emb in embeddings]
            
        except Exception as e:
            logger.error(f"Error generating dense embeddings batch: {e}")
            raise
    
    def _get_sparse_embedding(self, query: str) -> Tuple[np.ndarray, np.ndarray]:
        try:
            sparse_result = list(self.sparse_model.embed([query]))[0]
            indices = np.array(sparse_result.indices, dtype=np.uint32)
            values = np.array(sparse_result.values, dtype=np.float32)
            return indices, values
        except Exception as e:
            logger.error(f"Error generating sparse embedding: {e}")
            raise
    
    def _get_sparse_embeddings_batch(self, queries: List[str]) -> List[Tuple[np.ndarray, np.ndarray]]:
        try:
            query_indices = []
            query_texts = []
            passage_indices = []
            passage_texts = []
            prefix = "_Query_"
            
            for idx, q in enumerate(queries):
                if q.startswith(prefix):
                    query_indices.append(idx)
                    query_texts.append(q[len(prefix):])
                else:
                    passage_indices.append(idx)
                    passage_texts.append(q)

            all_sparse_results = [None] * len(queries)

            if query_texts:
                q_results = list(self.sparse_model.query_embed(query_texts))
                for i, res in enumerate(q_results):
                    original_idx = query_indices[i]
                    all_sparse_results[original_idx] = res

            if passage_texts:
                p_results = list(self.sparse_model.passage_embed(passage_texts))
                for i, res in enumerate(p_results):
                    original_idx = passage_indices[i]
                    all_sparse_results[original_idx] = res

            embeddings = []
            for sparse_result in all_sparse_results:
                if sparse_result is None:
                    logger.error("Encountered None in sparse results reconstruction")
                    raise ValueError("Sparse embedding generation failed for an item")

                indices = np.array(sparse_result.indices, dtype=np.uint32)
                values = np.array(sparse_result.values, dtype=np.float32)
                embeddings.append((indices, values))
            
            return embeddings
                
        except Exception as e:
            logger.error(f"Error generating sparse embeddings batch: {e}")
            raise
    
    def CreateEmbeddings(self, request, context):
        try:
            queries = request.queries
            if not queries:
                context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
                context.set_details("No queries provided")
                return embeddingService_pb2.Embeddings()
            
            _start_time = time.time()

            dense_future = self.executor.submit(self._get_dense_embeddings_batch, queries)
            sparse_future = self.executor.submit(self._get_sparse_embeddings_batch, queries)
            
            dense_embeddings_data = dense_future.result()
            sparse_embeddings_data = sparse_future.result()
            
            dense_embeddings = []
            for dense_emb in dense_embeddings_data:
                dense_embeddings.append(embeddingService_pb2.DenseEmbedding(values=dense_emb.tolist()))
            
            sparse_embeddings = []
            for indices, values in sparse_embeddings_data:
                sparse_embeddings.append(embeddingService_pb2.SparseEmbedding(
                    indices=indices.tolist(),
                    values=values.tolist()
                ))
            
            _elapsed_ms = (time.time() - _start_time) * 1000
            logger.info(f"CreateEmbeddings completed in {_elapsed_ms:.2f}ms for {len(queries)} queries")
            
            return embeddingService_pb2.Embeddings(
                dense_embeddings=dense_embeddings,
                sparse_embeddings=sparse_embeddings
            )
            
        except Exception as e:
            logger.error(f"Error in CreateEmbeddings: {e}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(e))
            return embeddingService_pb2.Embeddings()
    
    def CreateDenseEmbedding(self, request, context):
        try:
            query = request.query
            if not query or not query.strip():
                context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
                context.set_details("Empty query provided")
                return embeddingService_pb2.DenseEmbedding()
            
            _start_time = time.time()
            dense_emb = self._get_dense_embedding(query)
            _elapsed_ms = (time.time() - _start_time) * 1000
            logger.info(f"CreateDenseEmbedding completed in {_elapsed_ms:.2f}ms")
            
            return embeddingService_pb2.DenseEmbedding(values=dense_emb.tolist())
            
        except Exception as e:
            logger.error(f"Error in CreateDenseEmbedding: {e}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(e))
            return embeddingService_pb2.DenseEmbedding()


load_dotenv()
port = os.getenv("PORT", "50051")

def serve():
    # Dynamically grab the hardware allocation
    model_threads, executor_workers, grpc_workers = get_optimal_thread_config()

    # Pass grpc_workers to the front-door server
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=grpc_workers))
    
    # Pass model/executor allocation to the internal engine
    embeddingService_pb2_grpc.add_EmbeddingServiceServicer_to_server(
        EmbeddingServiceServicer(model_threads, executor_workers), server
    )
    
    server.add_insecure_port(f'[::]:{port}')
    
    logger.info(f"Starting gRPC server on port {port}...")
    server.start()
    logger.info(f"Server started successfully. Listening on port {port}")
    
    try:
        server.wait_for_termination()
    except KeyboardInterrupt:
        logger.info("Shutting down server...")
        server.stop(0)


if __name__ == '__main__':
    serve()