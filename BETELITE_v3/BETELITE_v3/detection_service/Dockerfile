FROM python:3.10-slim

WORKDIR /app

# Install system dependencies for OpenCV
RUN apt-get update && apt-get install -y libgl1-mesa-glx libglib2.0-0 && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Create a writable directory for EasyOCR models (useful for Hugging Face Spaces)
RUN mkdir -p /app/.EasyOCR && chmod 777 /app/.EasyOCR
ENV EASYOCR_MODULE_PATH=/app/.EasyOCR

COPY . .

# Expose port 8000 for Hugging Face Spaces / Render
EXPOSE 8000

# Start the FastAPI app
CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8000"]
