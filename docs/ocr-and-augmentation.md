# OCR support and the 150-image preparation pack

The extraction service can optionally attach local OCR observations to the existing vision-model request. Image/PDF uploads and the human-confirmation workflow keep the same contract. OCR does not create invoice references, approve invoices or post journal entries.

## Runtime pipeline

Enable `OCR_ENABLED=1` with `EXTRACTOR_MODE=ollama` in an environment containing the optional `ocr` dependencies (`pip install -e './ai-service[ocr]'`). Install the EasyOCR CRAFT detector and Latin generation-2 recognizer in advance, using the engine's documented model installation procedure. Runtime downloads are disabled. `OCR_MODEL_DIR` can point at their directory; by default it is `~/.EasyOCR/model`. The verified local machine already has `craft_mlt_25k.pth` and `latin_g2.pth`. Set `OCR_GPU=1` only when GPU acceleration is desired; CPU and two Torch threads are the default.

For each image (or each of at most five rendered PDF pages), the pipeline applies EXIF orientation, limits the longest edge to 1600 pixels and rejects decoded inputs above 20 million pixels. It uses grayscale/autocontrast preprocessing, followed by a full-image pass and overlapping header/footer passes with bounded magnification. It retains recognized text, confidence scores and boxes in the resized, oriented image coordinate space. These boxes are not source annotation boxes.

The model receives the original image pages plus bounded OCR observations marked as untrusted document content. OCR can be incorrect; the model is instructed to compare observations with the image. The service reports `extraction_mode=ollama_ocr` and adds a human-review warning. Missing engine dependencies/models produce a 503 rather than silently falling back to fabricated data. Readiness checks load the engine under its inference lock. The local `.env` remains in its existing demo configuration; enabling the pipeline requires the real extractor configuration and a compatible runtime.

The complete extraction request has a 90-second deadline, inside the Go client's existing 100-second HTTP timeout. Deadline expiry returns 504 and never posts business data. A running CPU OCR call can finish after cancellation, but no further pages or model request are scheduled for that cancelled extraction.

Engine parameters follow the official API: https://www.jaided.ai/easyocr/documentation/ . EasyOCR is pinned to 1.7.2 as an optional dependency, leaving the default lightweight service installation unchanged.

## Dataset

The prepared pack is `D:/PanlyackERP-training/dataset-150`. It contains 85 original supplied synthetic invoice/receipt pairs and 65 photometric variants, not 150 independent transactions. Raw source annotation snapshots are copied under `source_annotations/`; per-image labels point to the original record through `source_id` and preserve its header target. No archival class-only scan is added as a field reference.

Variants use Gaussian blur radius 0.35–0.55 pixels, brightness factors 0.96–1.02 and JPEG quality 94. They preserve dimensions and geometry, so the original positional labels remain applicable. These operations simulate mild capture degradation; they do not add new suppliers, layouts, amounts or language diversity. Supplied labels remain unconfirmed and should be visually reviewed after augmentation.

The eight original validation images from the first pilot remain unchanged. Only the other 77 original parents are eligible for augmentation. The resulting split is 142 training images and eight original validation images, with zero parent-image overlap. Supplier/layout independence is still not established.

Each OCR cache is independently generated from its image bytes, never from target labels. It contains the image SHA-256, OCR engine metadata, text, boxes and confidence scores. The preparation command checkpoints each image and can resume without repeating completed OCR work. The gallery links images, labels and OCR observations at `http://127.0.0.1:8095/` when the local pack server is running.

```powershell
# First preparation; refuses to overwrite an existing directory.
D:/PanlyackERP-training/venv/Scripts/python.exe scripts/prepare-ocr-dataset.py --with-ocr --gpu

# Resume only missing OCR observations.
D:/PanlyackERP-training/venv/Scripts/python.exe scripts/prepare-ocr-dataset.py --ocr-only --gpu

# Optional next training run: preserves the first adapter.
D:/PanlyackERP-training/venv/Scripts/python.exe scripts/train-light-invoices.py --dataset D:/PanlyackERP-training/dataset-150 --output D:/PanlyackERP-training/invoice-smolvlm-ocr-lora

# Evaluate an existing adapter with OCR context on the original held-out images.
D:/PanlyackERP-training/venv/Scripts/python.exe scripts/evaluate-light-invoices.py --run D:/PanlyackERP-training/invoice-smolvlm-lora --dataset D:/PanlyackERP-training/dataset-150
```

The training loader checks image hashes, OCR image hashes and parent separation before training. It supplies a bounded OCR header/footer context (up to 4000 characters) to limit memory use. It does not truncate image tokens. The first pilot's adapter and baseline generation report are preserved; OCR-assisted evaluation writes `ocr-generation-evaluation.json` separately. More augmented images do not establish better extraction accuracy: compare generated field values on the same original held-out images and retain human confirmation before ERP posting.

## Completed checks — 2026-10-17

All 150 images decoded and matched their recorded hashes. All 150 OCR caches matched their image hashes and contained nonempty text. Variant labels and dimensions matched their original parents; parent overlap between partitions was zero. OCR used the preinstalled EasyOCR detector/recognizer with GPU acceleration, approximately 1.6 GiB device memory when running alone. A sample original/variant pair was visually inspected; gallery and image/label/OCR HTTP links returned 200. Python tests: 42 passed, with the existing Starlette/httpx deprecation warning. The source/dataset integrity report is `D:/PanlyackERP-training/dataset-150/verification.json`.

An exploratory comparison reused the first pilot's adapter without retraining. On the same eight original held-out images, adding the bounded OCR transcript produced four parseable JSON objects and exact field matches number 0/8, supplier 1/8, printed date 3/8 and total 0/8. Baseline image-only results were five parseable JSON objects and matches 0/8, 2/8, 4/8 and 1/8 respectively. This is not an improvement. Initial structured-context diagnostics were stopped after observing the small model copying OCR observation fields; those traces were retained outside Git. The final comparison uses a quoted text transcript and repeats the requested output fields after it.

The OCR pipeline and augmented pack are delivered, but the existing adapter has not learned to use OCR context. No second fine-tuning run has been launched and no adapter is promoted to ERP extraction. The optional Ollama pipeline uses its existing constrained invoice schema; the exploratory SmolVLM generation comparison does not. Real OCR-plus-Ollama extraction quality and new-layout generalization remain unmeasured. Preparation data, caches, diagnostics, model outputs and local HTTP processes stay outside Git.
