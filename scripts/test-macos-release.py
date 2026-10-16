import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


class ReleaseVerificationTests(unittest.TestCase):
    def run_gate(self, **overrides):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "Rainforest.app").mkdir()
            (root / "Rainforest.dmg").touch()
            log = root / "calls"
            for name in ["codesign", "xcrun", "spctl"]:
                tool = root / name
                tool.write_text('''#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
name = Path(sys.argv[0]).name
with open(os.environ["CALL_LOG"], "a") as log:
    log.write(json.dumps([name, *sys.argv[1:]]) + "\\n")
if name == "codesign" and sys.argv[1] == "--display":
    print("TeamIdentifier=" + os.environ["SIGNING_TEAM"], file=sys.stderr)
    print("Authority=Developer ID Application: Test", file=sys.stderr)
if name == "codesign" and sys.argv[1] == "--verify":
    sys.exit(int(os.environ["VERIFY_EXIT"]))
if name == "xcrun":
    sys.exit(int(os.environ["STAPLER_EXIT"]))
if name == "spctl":
    sys.exit(int(os.environ["ASSESS_EXIT"]))
''')
                tool.chmod(0o755)
            env = dict(os.environ, PATH=f"{root}:{os.environ['PATH']}",
                       CALL_LOG=str(log), APPLE_TEAM_ID="TESTTEAM", SIGNING_TEAM="TESTTEAM",
                       VERIFY_EXIT="0", STAPLER_EXIT="0", ASSESS_EXIT="0")
            env.update(overrides)
            result = subprocess.run(["bash", str(Path(__file__).with_name("verify-macos-release.sh")),
                                     str(root)], env=env, capture_output=True, text=True)
            calls = [json.loads(line) for line in log.read_text().splitlines()] if log.exists() else []
            return result, calls

    def test_app_and_dmg_require_valid_signatures_tickets_and_gatekeeper(self):
        result, calls = self.run_gate()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(sum(call[:2] == ["codesign", "--verify"] for call in calls), 2)
        self.assertEqual(sum(call[:3] == ["xcrun", "stapler", "validate"] for call in calls), 2)
        assessments = [call for call in calls if call[0] == "spctl"]
        self.assertEqual(len(assessments), 2)
        self.assertIn("execute", assessments[0])
        self.assertIn("open", assessments[1])
        self.assertFalse(any("--sign" in call or "staple" in call for call in calls))

    def test_invalid_signature_stops_before_ticket_checks(self):
        result, calls = self.run_gate(VERIFY_EXIT="1")
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(any(call[0] == "xcrun" for call in calls))

    def test_wrong_team_stops_before_ticket_checks(self):
        result, calls = self.run_gate(SIGNING_TEAM="WRONGTEAM")
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(any(call[0] == "xcrun" for call in calls))

    def test_missing_ticket_stops_before_gatekeeper(self):
        result, calls = self.run_gate(STAPLER_EXIT="1")
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(any(call[0] == "spctl" for call in calls))

    def test_gatekeeper_rejection_fails_release(self):
        result, _ = self.run_gate(ASSESS_EXIT="1")
        self.assertNotEqual(result.returncode, 0)


if __name__ == "__main__":
    unittest.main()
