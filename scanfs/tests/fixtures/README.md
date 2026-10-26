# Unsupported JPEG fixture

Synthetic 9x8 alternating grayscale gradient, converted from the tracked
`crates/phash/tests/fixtures/rgb.png` with Pillow 11.3.0:

```python
Image.open("rgb.png").save("unsupported.jpg", format="JPEG", quality=90,
                          optimize=False, progressive=False)
```

No personal input, EXIF or ICC metadata. This valid JPEG verifies that enabling
PNG does not silently enable other codecs. SHA-256: `09bb0fa1a95e517225bc93b66c371dfd12c08eaae9f96a9955a00e4c37335bb8`.
Pillow is fixture-generation tooling, not a Scanner dependency.
