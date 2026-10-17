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

That comparison preceded the OCR training run recorded below. No adapter is promoted to ERP extraction. The optional Ollama pipeline uses its existing constrained invoice schema; the exploratory SmolVLM generation comparison does not. Real OCR-plus-Ollama extraction quality and new-layout generalization remain unmeasured. Preparation data, caches, diagnostics, model outputs and local HTTP processes stay outside Git.

## Completed OCR-assisted fine-tuning — 2026-10-17

Run directory: `D:/PanlyackERP-training/invoice-smolvlm-ocr-lora`. The original image-only adapter remains unchanged. Training used the expanded pack's 142 training images (77 original parents and 65 mild variants), with the same eight original validation images. The 108 test documents were excluded from optimization and generation evaluation; the loader checked their metadata and image integrity only. Supplied synthetic field labels remain unconfirmed by a human.

The pinned SmolVLM-256M base revision is `7e3e67edbbed1bf9888184d9df282b700a323964`; its weight checksum was verified before training. Three epochs ran on the local RTX 4060 Laptop GPU with BF16, batch size one, gradient accumulation four, learning rate `1e-4`, and rank-eight LoRA on language attention query/value projections. Only 460,800 parameters were trainable. No dependency or training-script changes were needed. Training took approximately 14 minutes; peak Torch tensor allocation was 5.764 GiB, while observed total device use reached approximately 7.9 GB. The saved adapter weights occupy 1,860,096 bytes; processor files and the base model are required separately.

| Checkpoint | Validation loss |
| --- | ---: |
| Before training | 1.584853 |
| Epoch 1 | 0.895696 |
| Epoch 2 | 0.487637 |
| Epoch 3, saved | 0.362865 |

Deterministic generation on the eight validation images used the same image-plus-OCR prompt as training, with a 160-token output limit. Counts below compare values exactly with supplied target strings; JSON validity means a parseable object, not full ERP schema validation.

| Measure | Original image-only pilot | New OCR-trained adapter |
| --- | ---: | ---: |
| Parseable JSON objects | 5/8 | 8/8 |
| Invoice number | 0/8 | 3/8 |
| Supplier | 2/8 | 5/8 |
| Printed date | 4/8 | 8/8 |
| Total, exact string | 1/8 | 5/8 |

One total mismatch is formatting only (`35.50` versus `35.5`); two other totals are materially wrong. Some predictions confuse telephone numbers with invoice numbers, or payment/address text with suppliers. This is a comparison of two complete pilot setups, not a controlled measurement of OCR's isolated contribution: augmentation and training exposure also changed. The small synthetic validation set reuses generator layouts and cannot establish real-document accuracy. Only four header fields were trained, with no line-item or accounting classification training.

The completed training report is `report.json`; all eight predictions are retained in `ocr-generation-evaluation.json` in the run directory. Standard output/error logs are `D:/PanlyackERP-training/ocr-training.*.log` and `ocr-evaluation.*.log`. The run remains experimental (`production_ready=false`), is not loaded into the ERP, and requires human confirmation before any business posting. The next evaluation can use the reserved test partition after freezing this checkpoint; no test-driven tuning has been performed.

```powershell
# Reproduce only validation generation for the completed run.
D:/PanlyackERP-training/venv/Scripts/python.exe scripts/evaluate-light-invoices.py --run D:/PanlyackERP-training/invoice-smolvlm-ocr-lora --dataset D:/PanlyackERP-training/dataset-expanded --split validation
```

The existing training/evaluation scripts executed successfully; no software test suite was rerun for this documentation-only update.

## Follow-up corrections — 2026-10-17

The optional Ollama OCR prompt now carries up to 48 recognized lines per page, with normalized `[left, top, right, bottom]` bounds in the EXIF-oriented resized page coordinate space. It retains observations from both the beginning and end of long pages. The combined text/geometry payload remains valid JSON capped at 16,000 characters across at most five pages. Prompt guidance distinguishes invoice numbers from telephone/order numbers, suppliers from customers/payment text, and final payable totals from subtotals/tax/paid amounts. No source annotation boxes or target values enter this evidence. This change affects the optional runtime OCR prompt only: the saved SmolVLM adapter and its training/evaluation prompt remain unchanged, and extraction still requires human confirmation. Quality improvement from the new runtime prompt has not been measured on real invoices.

Pilot scoring now reports `normalized_fields` alongside the original `exact_fields`. Only schema-compatible decimal strings are compared with `Decimal` equality for the total; floats, currency text, comma notation, scientific notation and nonfinite values are rejected. Other fields remain exact. The saved eight-document predictions can be rescored offline without model execution:

```powershell
.venv/Scripts/python.exe scripts/evaluate-light-invoices.py --rescore D:/PanlyackERP-training/invoice-smolvlm-ocr-lora/ocr-generation-evaluation.json
```

This writes a separate `ocr-generation-evaluation-normalized.json` and refuses to overwrite it. The original report/predictions remain intact. Numerically correct totals are 6/8, versus 5/8 exact strings; this corrects a metric, not the model. The other counts remain number 3/8, supplier 5/8 and date 8/8. The 108 test images were not evaluated.

Codex visually inspected all eight validation images and recorded the image hashes and per-field observations in `D:/PanlyackERP-training/invoice-smolvlm-ocr-lora/visual-label-audit.json`. This is agent inspection, not human-confirmed reference data. Invoices 001, 002 and 006 visibly print `id-997`, `id-925` and `id-222`; those supplied invoice numbers are not erroneous internal identifiers. The supplier headings of invoices 003 and 005 are cropped out or unreadable at the top edge; invoice 004 has a partially visible heading at the top edge. Their supplied supplier values should not be treated as fully observable extraction references without an uncropped image or a separately versioned field-visibility review. Printed numbers, dates and totals agree with supplied targets on the eight inspected images. No labels, original images, frozen dataset partitions or trained weights were automatically replaced, and the training partition has not received this visual audit.

Verification: 50 Python tests passed, including the API request carrying OCR bounds, bounded evidence serialization, source-label isolation, exact decimal precision and non-destructive offline rescoring. The existing Starlette/httpx deprecation warning remains. These checks establish pipeline behavior, not model accuracy.
