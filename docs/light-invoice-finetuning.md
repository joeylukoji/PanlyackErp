# Lightweight invoice fine-tuning pilot

The local hardware audit found an AMD Ryzen 7 8845HS (8 cores, 16 threads), approximately 15.3 GiB physical RAM and an NVIDIA RTX 4060 Laptop GPU with 8 GiB VRAM. CUDA is available in the existing PyTorch 2.6.0+cu124 installation. C: has only approximately 2.34 GiB free; D: has approximately 88 GiB free. Training dependencies and model artifacts therefore live under `D:/PanlyackERP-training`.

This pilot uses `HuggingFaceTB/SmolVLM-256M-Instruct`, with rank-8 LoRA on language attention query/value projections, BF16, one image per forward pass, gradient accumulation of four, three epochs and a learning rate of 0.0001. The base model remains frozen. Only the adapter is saved; the ERP extractor configuration is unchanged.

Run with the separate environment:

```powershell
D:/PanlyackERP-training/venv/Scripts/python.exe scripts/train-light-invoices.py
```

The environment reuses installed PyTorch through `--system-site-packages`; additional packages are Transformers 4.57.1, PEFT 0.17.1 and Accelerate 1.10.1. No global packages are replaced.

Only supplied synthetic image/label pairs are eligible: the 50 local receipts and 35 supplier invoices. Public archival scans have no field references and are excluded, including the five documents whose class was reviewed. Header targets are `number`, `partner`, `issue_date` (as printed) and `total` (decimal string). This is a header extraction experiment, not full line-item training. Source labels remain identified as supplied synthetic labels; they are never marked as human-confirmed extraction references.

The base revision is pinned to `7e3e67edbbed1bf9888184d9df282b700a323964`. Before model loading, the script requires weights SHA-256 `74dea5904032e5ae99a2e0eef5179e6ac0f1dedc3ab0c7c2a5d4d387c843203e`. An interrupted Xet/cache download produced a readable but corrupt checkpoint with zero embeddings, which caused uniform loss. That diagnostic attempt was stopped without saving an adapter. The script now resumes a separate HTTP download and replaces the cached weights only after verifying the full hash. Nonzero adapter gradients are also required before every optimizer step.

The seeded split uses distinct original image hashes and holds out approximately ten percent for validation. Templates can appear in both partitions, so held-out loss is an in-distribution diagnostic, not evidence of generalization to new suppliers. The run writes its exact split, validation loss before/after fine-tuning, GPU peak allocation and adapter. An adapter is not production-ready based on loss alone: visible field agreement and held-out real-document extraction accuracy still require human-confirmed references.

After training completes, run `D:/PanlyackERP-training/venv/Scripts/python.exe scripts/evaluate-light-invoices.py` to record deterministic generated JSON and exact field matches on the eight held-out synthetic images. Invalid JSON counts as a failed field match. Generated outputs and expected values are retained for inspection in `generation-evaluation.json`; this measures only the supplied labels and the stated split.

Runtime logs are `D:/PanlyackERP-training/training.stdout.log` and `training.stderr.log`. A completed run has `invoice-smolvlm-lora/report.json` with `completed: true` and an `adapter/` directory. A split file or download log alone does not mean training completed. The script permits retrying a preparation-only directory but refuses to overwrite an existing adapter or completed report.

Official model documentation: https://huggingface.co/HuggingFaceTB/SmolVLM-256M-Instruct

OCR context and a 150-image preparation pack are documented in [OCR support and augmentation](ocr-and-augmentation.md). The training/evaluation scripts now accept an explicit dataset and output directory so the first pilot remains preserved. A new training run on that pack has not been started.

## Completed local experiment — 2026-10-17

The verified model completed all three epochs on 77 training images with eight held-out images. Baseline validation loss was 1.8270; losses after each epoch were 1.4323, 0.9906 and 0.6569. Peak tensor memory allocation measured by PyTorch was 3.609 GiB; observed total device memory including runtime reservations was approximately 4.3 GiB. The adapter weights are 1,860,096 bytes; tokenizer/processor files and the frozen base model are required separately.

Actual deterministic generation on the eight held-out images produced only five strictly parseable JSON objects. Exact field matches, counting invalid JSON as failures, were number 0/8, supplier 2/8, printed date 4/8 and total 1/8. Amounts with comma formatting or currency symbols are not counted as exact decimal-string matches. The per-image generated strings are preserved in `D:/PanlyackERP-training/invoice-smolvlm-lora/generation-evaluation.json`.

This is a completed training experiment with insufficient extraction quality for ERP use. The decreasing teacher-forced loss did not establish reliable free-generation extraction. The pilot is not enabled in the application. Further work must target the visible extraction failures and use stronger reviewed field references; repeating epochs or reporting the lower loss as extraction accuracy would be misleading.

Verification performed: original model SHA-256 check, 85 unique decodable image/label pairs, required number/total targets, nonzero adapter gradients, finite training losses, adapter save and reload, held-out generation, and 33 passing Python tests. The existing Starlette/httpx deprecation warning remains. No accuracy on real external invoices or unseen layouts has been measured.
