#!/usr/bin/env python3
"""Run deterministic real-filesystem catalog probes without editing source files.

Go's overlay maps an evidence-owned test into the existing internal scan package.
Only synthetic temporary files are removed; no runtime hook enters production.
"""
import argparse
import hashlib
import json
from pathlib import Path
import platform
import subprocess

PROBE = r'''package scan

import (
 "context"
 "errors"
 "os"
 "path/filepath"
 "testing"
 "scan-projects/internal/scan/checkpoint"
)

func TestNativeQualificationDisappearingFile(t *testing.T) {
 root := t.TempDir()
 first, vanished := filepath.Join(root,"a-first.txt"), filepath.Join(root,"z-vanished.txt")
 for _, p := range []string{first, vanished} { if err:=os.WriteFile(p,[]byte("synthetic"),0600);err!=nil {t.Fatal(err)} }
 warned, removed := false, false
 result, err := Scan(context.Background(), []string{root}, nil, ScanOptions{
  HashFiles:true,
  OnCheckpoint:func(p string){if p==first {if e:=os.Remove(vanished);e!=nil{t.Fatal(e)};removed=true}},
  OnWarning:func(p string,e error){if p==vanished && errors.Is(e,os.ErrNotExist){warned=true}},
 }, "")
 valid := len(result.Files)>=1 && result.Files[0].Path==first && result.Files[0].HashStatus=="ok"
 // Windows caches DirEntry metadata: a disappeared file can survive as an explicit hash error.
 for _,r:=range result.Files[1:] {if r.Path!=vanished || r.HashStatus!="error" || r.Sha256!="" {valid=false}}
 if err!=nil || !removed || !warned || !valid || len(result.Files)>2 {
  t.Fatalf("err=%v removed=%v warned=%v files=%+v",err,removed,warned,result.Files)
 }
}

func TestNativeQualificationCancellationAtCheckpoint(t *testing.T) {
 root:=t.TempDir()
 for _, name:=range []string{"a.txt","b.txt","c.txt"}{if e:=os.WriteFile(filepath.Join(root,name),[]byte("synthetic"),0600);e!=nil{t.Fatal(e)}}
 ctx,cancel:=context.WithCancel(context.Background());defer cancel()
 result,err:=Scan(ctx,[]string{root},nil,ScanOptions{HashFiles:true,OnCheckpoint:func(string){cancel()}},"")
 if !errors.Is(err,context.Canceled)||len(result.Files)!=1||result.Files[0].HashStatus!="ok" {t.Fatalf("err=%v files=%+v",err,result.Files)}
}

func TestNativeQualificationCheckpointInventory(t *testing.T) {
 root:=t.TempDir();cpPath:=filepath.Join(t.TempDir(),"checkpoint.json")
 for _,n:=range []string{"a.txt","b.txt","c.txt"}{if e:=os.WriteFile(filepath.Join(root,n),[]byte(n),0600);e!=nil{t.Fatal(e)}}
 full,e:=Scan(context.Background(),[]string{root},nil,ScanOptions{HashFiles:true},"");if e!=nil{t.Fatal(e)}
 ctx,cancel:=context.WithCancel(context.Background());defer cancel()
 prefix,e:=Scan(ctx,[]string{root},nil,ScanOptions{HashFiles:true,OnCheckpoint:func(p string){if e:=checkpoint.Save(cpPath,p);e!=nil{t.Fatal(e)};if filepath.Base(p)=="b.txt"{cancel()}}},"")
 if !errors.Is(e,context.Canceled)||len(prefix.Files)!=2{t.Fatalf("prefix %v %+v",e,prefix.Files)}
 cp,ok:=checkpoint.Load(cpPath);if !ok||filepath.Base(cp.LastPath)!="b.txt"{t.Fatalf("checkpoint %+v loaded=%v",cp,ok)}
 suffix,e:=Scan(context.Background(),[]string{root},nil,ScanOptions{HashFiles:true},cp.LastPath)
 if e!=nil||len(suffix.Files)!=2||filepath.Base(suffix.Files[0].Path)!="b.txt"{t.Fatalf("inclusive suffix %v %+v",e,suffix.Files)}
 merged:=map[string]FileRecord{};for _,part:=range [][]FileRecord{prefix.Files,suffix.Files}{for _,r:=range part{merged[r.Path]=r}}
 if len(merged)!=len(full.Files){t.Fatalf("merged=%d full=%d",len(merged),len(full.Files))}
 for _,r:=range full.Files{got,ok:=merged[r.Path];if !ok||got.Sha256!=r.Sha256||got.SizeBytes!=r.SizeBytes||got.HashStatus!=r.HashStatus{t.Fatalf("inventory mismatch at %s",r.Path)}}
 // Folder aggregates remain partial on suffix resume; the cursor is not a full-result snapshot.
}

func TestNativeQualificationSymlink(t *testing.T) {
 root:=t.TempDir();target:=filepath.Join(root,"target.txt");link:=filepath.Join(root,"linked.txt")
 if err:=os.WriteFile(target,[]byte("synthetic symlink content"),0600);err!=nil{t.Fatal(err)}
 if err:=os.Symlink("target.txt",link);err!=nil{t.Fatalf("native symlink creation unavailable: %v",err)}
 result,err:=Scan(context.Background(),[]string{root},nil,ScanOptions{HashFiles:true},"")
 if err!=nil||len(result.Files)!=2{t.Fatalf("err=%v files=%+v",err,result.Files)}
 records:=map[string]FileRecord{};for _,r:=range result.Files{records[r.Path]=r}
 a,b:=records[target],records[link]
 if a.HashStatus!="ok"||b.HashStatus!="ok"||a.Sha256==""||a.Sha256!=b.Sha256 {t.Fatalf("target=%+v link=%+v",a,b)}
 // Compatibility records use link lstat size while hashing target content.
 info,e:=os.Lstat(link);if e!=nil||b.SizeBytes!=info.Size(){t.Fatalf("link size mismatch: %v %+v",e,b)}
}
'''


EXPECTED = {"TestNativeQualificationDisappearingFile", "TestNativeQualificationCancellationAtCheckpoint",
            "TestNativeQualificationSymlink", "TestNativeQualificationCheckpointInventory"}


def summarize(stdout, returncode):
    tests = {}
    for line in stdout.splitlines():
        value = json.loads(line)
        if value.get("Test") and value["Action"] in ("pass", "fail", "skip"):
            tests[value["Test"]] = value["Action"]
    return tests, returncode == 0 and set(tests) == EXPECTED and all(v == "pass" for v in tests.values())


def run(source, evidence, go="go"):
    source, evidence = Path(source).resolve(), Path(evidence).resolve()
    if evidence.is_relative_to(source):
        raise ValueError("evidence must be outside the checkout")
    evidence.mkdir(parents=True, exist_ok=False)
    probe = evidence / "native_qualification_test.go"
    probe.write_text(PROBE, encoding="utf-8")
    overlay = evidence / "overlay.json"
    virtual = source / "tools/project-catalog/internal/scan/native_qualification_test.go"
    overlay.write_text(json.dumps({"Replace": {str(virtual): str(probe)}}), encoding="utf-8")
    command = [go, "test", "-json", "-count=1", "-overlay", str(overlay),
               "./internal/scan", "-run", "^TestNativeQualification"]
    process = subprocess.run(command, cwd=source / "tools/project-catalog", capture_output=True,
                             text=True, encoding="utf-8", timeout=120)
    (evidence / "go-test.jsonl").write_text(process.stdout, encoding="utf-8")
    (evidence / "go-test.stderr.txt").write_text(process.stderr, encoding="utf-8")
    tests, passed = summarize(process.stdout, process.returncode)
    report = {"schema": 1, "system": platform.system(), "architecture": platform.machine(),
              "probe_sha256": hashlib.sha256(PROBE.encode()).hexdigest(), "tests": tests,
              "exit_code": process.returncode, "passed": passed,
              "signal_qualification": "separate-native-process-gate",
              "release_acceptance": False}
    (evidence / "report.json").write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    return report


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, required=True)
    parser.add_argument("--evidence", type=Path, required=True)
    parser.add_argument("--go", default="go")
    args = parser.parse_args()
    result = run(args.source, args.evidence, args.go)
    print(json.dumps(result, indent=2))
    raise SystemExit(0 if result["passed"] else 1)
