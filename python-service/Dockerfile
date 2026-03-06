FROM python:3.12.3-slim

WORKDIR /app

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

ENV PORT=50051

COPY req.txt .
RUN pip install --no-cache-dir -r req.txt

COPY . .

EXPOSE 50051

CMD ["python3", "main.py"]