# ADR 0013: Decisions from `assets/visual-system/pipeline/render_tgs_preview.py`

Status: Accepted

These decisions were written as comments in `assets/visual-system/pipeline/render_tgs_preview.py` and moved here so the source no longer carries them. Each entry is the original comment.

## The repository is work/The-Vagabond, so toolcache is its direct sibling.

Removed from `assets/visual-system/pipeline/render_tgs_preview.py:22`.

~~~~text
The repository is work/The-Vagabond, so toolcache is its direct sibling.
~~~~


## Chrome's Windows launcher can return before its screenshot child has

Removed from `assets/visual-system/pipeline/render_tgs_preview.py:69`.

~~~~text
Chrome's Windows launcher can return before its screenshot child has
written the file, so wait on the output rather than assuming the
launcher process lifetime matches the renderer lifetime.
~~~~
