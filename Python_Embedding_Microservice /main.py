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
            embedding = list(self.dense_model([query]))[0]
            return embedding.astype(np.float32)
        except Exception as e:
            logger.error(f"Error generating dense embedding: {e}")
            raise
    
    def _get_dense_embeddings_batch(self, queries: List[str]) -> List[np.ndarray]:
        """
        Generate dense embeddings for multiple queries in a single batch
        Handles '_Query_' prefix logic for BGE instruction.
        """
        try:
            # PROCESS INPUTS: Handle prefix logic
            processed_queries = []
            bge_instruction = "Represent this sentence for searching relevant passages: "
            prefix = "_Query_"
            
            for q in queries:
                if q.startswith(prefix):
                    # 1. Trim the prefix
                    trimmed_q = q[len(prefix):]
                    # 2. Prepend BGE instruction (Only for dense)
                    processed_queries.append(bge_instruction + trimmed_q)
                else:
                    # Passage: Leave as is
                    processed_queries.append(q)

            # Pass the processed list to the model
            embeddings = list(self.dense_model.embed(processed_queries))
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
        Generate sparse embeddings for multiple queries in a single batch.
        
        Splits the batch into 'queries' and 'passages' based on the '_Query_' prefix
        to utilize the model's specialized query_embed vs passage_embed methods.
        """
        try:
            # 1. SEGREGATION
            # We need to track original indices to reconstruct the order later
            query_indices = []
            query_texts = []
            
            passage_indices = []
            passage_texts = []
            
            prefix = "_Query_"
            
            for idx, q in enumerate(queries):
                if q.startswith(prefix):
                    # Found a Query: Trim prefix and add to query batch
                    query_indices.append(idx)
                    query_texts.append(q[len(prefix):])
                else:
                    # Found a Passage: Add as is
                    passage_indices.append(idx)
                    passage_texts.append(q)

            # Prepare a list to hold results in the correct order (None as placeholder)
            # This ensures index 0 in input maps to index 0 in output
            all_sparse_results = [None] * len(queries)

            # 2. EXECUTION
            # Process Queries (if any)
            if query_texts:
                # Using the specific query_embed method as requested
                q_results = list(self.sparse_model.query_embed(query_texts))
                for i, res in enumerate(q_results):
                    original_idx = query_indices[i]
                    all_sparse_results[original_idx] = res

            # Process Passages (if any)
            if passage_texts:
                # Using the specific passage_embed method as requested
                p_results = list(self.sparse_model.passage_embed(passage_texts))
                for i, res in enumerate(p_results):
                    original_idx = passage_indices[i]
                    all_sparse_results[original_idx] = res

            # 3. FORMATTING
            embeddings = []
            for sparse_result in all_sparse_results:
                if sparse_result is None:
                    # This should theoretically never happen given the logic above
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
            # Note: The input 'queries' list is passed to both; they handle their own
            # string processing internally so there are no race conditions.
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



# 1. Try to load .env file (useful for local dev, ignored if file missing)
load_dotenv()

# 2. Get PORT from Environment, default to 50051 if missing
port = os.getenv("PORT", "50051")

def serve(port=port, max_workers=10):
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