#!/usr/bin/env python3
"""Remove an image background with a preinstalled local u2netp model (GPU first).

Install scripts/matting-requirements.txt in .local/matting-venv and provision
u2netp.onnx into .local/matting-models before invoking this offline helper.
The model is never automatically downloaded by this command.
"""
from __future__ import annotations

import argparse
from contextlib import redirect_stderr, redirect_stdout
import hashlib
import io
import json
import os
from pathlib import Path
import sys

# Official rembg u2netp release checksum; validate before rembg can call pooch.
U2NETP_MD5 = "8e83ca70e441ab06c318d82300c84806"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--device", choices=("gpu", "cpu"), default="gpu",
                        help="Preferred device (default: gpu); retry on CPU if GPU fails")
    parser.add_argument("--input", required=True, type=Path, help="Source image path")
    parser.add_argument("--output", required=True, type=Path, help="Destination RGBA PNG path")
    parser.add_argument(
        "--model-dir", type=Path,
        default=Path(__file__).resolve().parent.parent / ".local" / "matting-models",
        help="Directory containing the pre-downloaded u2netp.onnx model",
    )
    return parser.parse_args()


CPU_PROVIDER = "CPUExecutionProvider"
GPU_PROVIDERS = ("CUDAExecutionProvider", "DmlExecutionProvider", "ROCMExecutionProvider")


def run_inference(source, model_path: Path, device: str):
    """Try one accelerator, then a fresh CPU session; never trust availability alone.

    Reasons are fixed codes, not exception text (which can expose local paths,
    credentials, control characters or native provider diagnostics).
    """
    import onnxruntime as ort
    from rembg import new_session, remove
    from rembg.sessions.u2netp import U2netpSession
    from unittest.mock import patch

    requested_provider = CPU_PROVIDER if device == "cpu" else None
    fallback_reason = None
    if device == "gpu":
        try:
            available = ort.get_available_providers()
            requested_provider = next((p for p in GPU_PROVIDERS if p in available), None)
            if requested_provider is None:
                fallback_reason = "gpu_provider_unavailable"
        except Exception:
            fallback_reason = "gpu_provider_discovery_failed"

    def attempt(provider):
        options = ort.SessionOptions()
        # Do not leak native provider diagnostics; report stable reason codes.
        options.log_severity_level = 4
        if provider == "DmlExecutionProvider":
            options.enable_mem_pattern = False
            options.execution_mode = ort.ExecutionMode.ORT_SEQUENTIAL
        providers = [provider] if provider == CPU_PROVIDER else [provider, CPU_PROVIDER]
        # Bind every attempt to the already checksum-verified local model. No
        # session constructor may resolve another cache or trigger a download.
        with redirect_stdout(io.StringIO()), redirect_stderr(io.StringIO()):
            with patch.object(U2netpSession, "download_models", return_value=str(model_path)):
                session = new_session("u2netp", providers=providers, sess_opts=options)
            actual = session.inner_session.get_providers()
            if provider not in actual:
                return None, None, "gpu_provider_not_active"
            # ORT otherwise can silently switch to CPU when Run raises EPFail.
            session.inner_session.disable_fallback()
            try:
                result = remove(source.copy(), session=session).convert("RGBA")
            except Exception:
                if provider == CPU_PROVIDER:
                    raise
                return None, None, "gpu_inference_failed"
            actual = session.inner_session.get_providers()
            if provider not in actual:
                return None, None, "gpu_provider_changed_during_inference"
            return result, actual, None

    if device == "gpu" and requested_provider is not None:
        try:
            result, actual, fallback_reason = attempt(requested_provider)
            if fallback_reason is None:
                return result, requested_provider, requested_provider, actual, None
        except Exception:
            fallback_reason = "gpu_session_failed"
    result, actual, failure = attempt(CPU_PROVIDER)
    if failure is not None:
        raise RuntimeError("CPU provider not active")
    return result, requested_provider, CPU_PROVIDER, actual, fallback_reason


def main() -> int:
    args = parse_args()
    model_dir = args.model_dir.resolve()
    model_path = model_dir / "u2netp.onnx"
    if not model_path.is_file():
        print(f"Local model missing: {model_path}. Provision the official rembg u2netp model first; automatic downloads are disabled.", file=sys.stderr)
        return 2
    if hashlib.md5(model_path.read_bytes()).hexdigest() != U2NETP_MD5:
        print(f"Local model checksum mismatch: {model_path}. Restore the official rembg u2netp model; automatic downloads are disabled.", file=sys.stderr)
        return 2
    if not args.input.is_file():
        print(f"Input image not found: {args.input}", file=sys.stderr)
        return 2
    if args.input.resolve() == args.output.resolve():
        print("Input and output must be different paths.", file=sys.stderr)
        return 2
    if args.output.suffix.lower() != ".png":
        print("Output must use the .png extension (RGBA PNG).", file=sys.stderr)
        return 2
    os.environ["U2NET_HOME"] = str(model_dir)
    try:
        from PIL import Image
        with Image.open(args.input) as source:
            source.load()
            result, requested, actual, active, reason = run_inference(
                source.convert("RGBA"), model_path, args.device)

        args.output.parent.mkdir(parents=True, exist_ok=True)
        result.save(args.output, format="PNG")
        alpha = result.getchannel("A")
        histogram = alpha.histogram()
        print(json.dumps({
            "output": str(args.output.resolve()),
            "model": "u2netp",
            "requested_device": args.device,
            "requested_provider": requested,
            "actual_device": "cpu" if actual == CPU_PROVIDER else "gpu",
            "actual_provider": actual,
            "provider": actual,
            "active_providers": active,
            "fallback_reason": reason,
            "mode": result.mode,
            "width": result.width,
            "height": result.height,
            "alpha_min": alpha.getextrema()[0],
            "alpha_max": alpha.getextrema()[1],
            "transparent_pixels": histogram[0],
            "opaque_pixels": histogram[255],
            "partial_alpha_pixels": sum(histogram[1:255]),
        }))
        return 0
    except Exception:
        print("Local background removal failed: local_runtime_or_image_error", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
