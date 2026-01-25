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
import asyncio
from concurrent.futures import ThreadPoolExecutor

from fastembed import TextEmbedding, SparseTextEmbedding

import embeddingService_pb2
import embeddingService_pb2_grpc

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class EmbeddingServiceServicer(embeddingService_pb2_grpc.EmbeddingServiceServicer):

    def __init__(self):
        logger.info("Initializing embedding models...")
        
        
        self.dense_model = TextEmbedding(
            model_name="BAAI/bge-small-en-v1.5",
            max_length=512,
            threads=2 
        )
        
        self.sparse_model = SparseTextEmbedding(
            model_name="prithivida/Splade_PP_en_v1",
            threads=2 
        )
        
        self.executor = ThreadPoolExecutor(max_workers=2)
    
    def _get_dense_embedding(self, query: str) -> np.ndarray:
        """
        Generate dense embedding for a single query
        
        Args:
            query: Input text string
            
        Returns:
            numpy array of dense embedding values
        """
        try:
            embedding = list(self.dense_model.embed([query]))[0]
            return embedding.astype(np.float32)
        except Exception as e:
            logger.error(f"Error generating dense embedding: {e}")
            raise
    
    def _get_dense_embeddings_batch(self, queries: List[str]) -> List[np.ndarray]:
        """
        Generate dense embeddings for multiple queries in a single batch
        
        Args:
            queries: List of input text strings
            
        Returns:
            List of numpy arrays of dense embedding values
        """
        try:
            embeddings = list(self.dense_model.embed(queries))
            return [emb.astype(np.float32) for emb in embeddings]
        except Exception as e:
            logger.error(f"Error generating dense embeddings batch: {e}")
            raise
    
    def _get_sparse_embedding(self, query: str) -> Tuple[np.ndarray, np.ndarray]:
        """
        Generate sparse embedding for a single query
        
        Args:
            query: Input text string
            
        Returns:
            tuple of (indices, values) where indices are token IDs and values are weights
        """
        try:
            sparse_result = list(self.sparse_model.embed([query]))[0]
            
            indices = np.array(sparse_result.indices, dtype=np.uint32)
            values = np.array(sparse_result.values, dtype=np.float32)
            
            return indices, values
                
        except Exception as e:
            logger.error(f"Error generating sparse embedding: {e}")
            raise
    
    def _get_sparse_embeddings_batch(self, queries: List[str]) -> List[Tuple[np.ndarray, np.ndarray]]:
        """
        Generate sparse embeddings for multiple queries in a single batch
        
        Args:
            queries: List of input text strings
            
        Returns:
            List of tuples (indices, values) for each query
        """
        try:
            sparse_results = list(self.sparse_model.embed(queries))
            
            embeddings = []
            for sparse_result in sparse_results:
                indices = np.array(sparse_result.indices, dtype=np.uint32)
                values = np.array(sparse_result.values, dtype=np.float32)
                embeddings.append((indices, values))
            
            return embeddings
                
        except Exception as e:
            logger.error(f"Error generating sparse embeddings batch: {e}")
            raise
    
    def CreateEmbeddings(self, request, context):
        """
        Create both dense and sparse embeddings for a list of queries
        
        Args:
            request: Queries message containing list of query strings
            context: gRPC context
            
        Returns:
            Embeddings message with dense and sparse embeddings
        """
        try:
            queries = request.queries
            logger.info(f"Received CreateEmbeddings request for {len(queries)} queries")
            
            if not queries:
                context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
                context.set_details("No queries provided")
                return embeddingService_pb2.Embeddings()
            
            # Run dense and sparse embeddings concurrently for maximum performance
            dense_future = self.executor.submit(self._get_dense_embeddings_batch, queries)
            sparse_future = self.executor.submit(self._get_sparse_embeddings_batch, queries)
            
            # Wait for both to complete
            dense_embeddings_data = dense_future.result()
            sparse_embeddings_data = sparse_future.result()
            
            # Convert to protobuf messages
            dense_embeddings = []
            for dense_emb in dense_embeddings_data:
                dense_embedding_msg = embeddingService_pb2.DenseEmbedding(
                    values=dense_emb.tolist()
                )
                dense_embeddings.append(dense_embedding_msg)
            
            sparse_embeddings = []
            for indices, values in sparse_embeddings_data:
                sparse_embedding_msg = embeddingService_pb2.SparseEmbedding(
                    indices=indices.tolist(),
                    values=values.tolist()
                )
                sparse_embeddings.append(sparse_embedding_msg)
            
            logger.info(f"Successfully created embeddings for {len(queries)} queries")
            
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
        """
        Create dense embedding for a single query
        
        Args:
            request: Query message containing a single query string
            context: gRPC context
            
        Returns:
            DenseEmbedding message with embedding values
        """
        try:
            query = request.query
            logger.info(f"Received CreateDenseEmbedding request for query: '{query[:50]}...'")
            
            if not query or not query.strip():
                context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
                context.set_details("Empty query provided")
                return embeddingService_pb2.DenseEmbedding()
            
            # Generate dense embedding
            dense_emb = self._get_dense_embedding(query)
            
            logger.info(f"Successfully created dense embedding (dim: {len(dense_emb)})")
            
            return embeddingService_pb2.DenseEmbedding(
                values=dense_emb.tolist()
            )
            
        except Exception as e:
            logger.error(f"Error in CreateDenseEmbedding: {e}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(e))
            return embeddingService_pb2.DenseEmbedding()


def serve(port=50051, max_workers=10):
    """
    Start the gRPC server
    
    Args:
        port: Port number to listen on
        max_workers: Maximum number of worker threads
    """
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=max_workers))
    
    # Add the servicer to the server
    embeddingService_pb2_grpc.add_EmbeddingServiceServicer_to_server(
        EmbeddingServiceServicer(), server
    )
    
    # Bind to port
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