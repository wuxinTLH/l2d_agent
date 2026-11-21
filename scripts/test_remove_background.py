"""Offline provider/fallback contract tests; no model or GPU required."""
import importlib.util
from pathlib import Path
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch

spec = importlib.util.spec_from_file_location("remove_background", Path(__file__).with_name("remove-background.py"))
helper = importlib.util.module_from_spec(spec)
spec.loader.exec_module(helper)
CPU = helper.CPU_PROVIDER
GPU = "DmlExecutionProvider"


class ProviderTests(unittest.TestCase):
    def run_fake(self, available=(GPU, CPU), failure=None, cpu_failure=False):
        calls = []
        inners = []
        options = []
        source = Mock()
        result = Mock()
        result.convert.return_value = result
        def opts():
            value = SimpleNamespace()
            options.append(value)
            return value
        def new_session(name, providers, sess_opts):
            self.assertEqual(model_class.download_models(), "verified.onnx")
            calls.append(providers)
            if providers[0] == GPU and failure == "session":
                raise RuntimeError("C:/secret/token?api_key=PRIVATE\nunsafe")
            inner = Mock()
            inner.get_providers.return_value = [CPU] if failure == "silent" else providers
            if failure == "changed" and providers[0] == GPU:
                inner.get_providers.side_effect = [providers, [CPU]]
            inners.append(inner)
            return SimpleNamespace(inner_session=inner)
        def remove(image, session):
            if cpu_failure or (failure == "inference" and session.inner_session.get_providers()[0] == GPU):
                raise RuntimeError("private native diagnostics")
            return result
        model_class = type("U2netpSession", (), {"download_models": Mock(side_effect=AssertionError("download forbidden"))})
        ort = SimpleNamespace(get_available_providers=Mock(return_value=available), SessionOptions=opts,
                              ExecutionMode=SimpleNamespace(ORT_SEQUENTIAL="sequential"))
        if failure == "discovery":
            ort.get_available_providers.side_effect = RuntimeError("PRIVATE")
        modules = {"onnxruntime": ort, "rembg": SimpleNamespace(new_session=new_session, remove=remove),
                   "rembg.sessions.u2netp": SimpleNamespace(U2netpSession=model_class)}
        with patch.dict(sys.modules, modules):
            values = helper.run_inference(source, Path("verified.onnx"), self.device)
        model_class.download_models.assert_not_called()
        return values, calls, inners, options

    device = "gpu"

    def test_gpu_default_cli(self):
        with patch.object(sys, "argv", ["helper", "--input", "a", "--output", "b.png"]):
            self.assertEqual(helper.parse_args().device, "gpu")

    def test_gpu_success(self):
        values, calls, inners, options = self.run_fake()
        self.assertEqual(values[1:], (GPU, GPU, [GPU, CPU], None))
        self.assertEqual(calls, [[GPU, CPU]])
        self.assertFalse(options[0].enable_mem_pattern)
        self.assertEqual(options[0].execution_mode, "sequential")
        inners[0].disable_fallback.assert_called_once()

    def test_cpu_explicit(self):
        self.device = "cpu"
        values, calls, _, _ = self.run_fake()
        self.assertEqual(calls, [[CPU]])
        self.assertEqual(values[1:], (CPU, CPU, [CPU], None))

    def test_unavailable(self):
        values, calls, _, _ = self.run_fake(available=[CPU])
        self.assertEqual(calls, [[CPU]])
        self.assertEqual(values[1:], (None, CPU, [CPU], "gpu_provider_unavailable"))

    def test_provider_discovery_failure(self):
        values, calls, _, _ = self.run_fake(failure="discovery")
        self.assertEqual(calls, [[CPU]])
        self.assertEqual(values[1:], (None, CPU, [CPU], "gpu_provider_discovery_failed"))

    def test_fallback_errors_are_sanitized(self):
        for failure, reason in (("session", "gpu_session_failed"), ("inference", "gpu_inference_failed"),
                                ("silent", "gpu_provider_not_active"),
                                ("changed", "gpu_provider_changed_during_inference")):
            with self.subTest(failure=failure):
                values, calls, _, _ = self.run_fake(failure=failure)
                self.assertEqual(calls, [[GPU, CPU], [CPU]])
                self.assertEqual(values[1:], (GPU, CPU, [CPU], reason))

    def test_cpu_error_is_not_retried(self):
        self.device = "cpu"
        with self.assertRaises(RuntimeError):
            self.run_fake(cpu_failure=True)

    def test_missing_or_bad_model_never_imports_runtime(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for bad_model in (False, True):
                if bad_model:
                    (root / "u2netp.onnx").write_bytes(b"invalid")
                with patch.object(sys, "argv", ["helper", "--input", "a", "--output", "b.png", "--model-dir", directory]), \
                     patch.object(helper, "run_inference", side_effect=AssertionError("inference forbidden")), \
                     patch("sys.stderr"):
                    self.assertEqual(helper.main(), 2)


if __name__ == "__main__":
    unittest.main()
