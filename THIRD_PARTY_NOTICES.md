# Third-party notices

**This project is licensed under the GNU General Public License v2** (see
LICENSE). That choice makes licensing unambiguous: the project embeds a GPLv2
binary (`oggenc2.exe`), so the whole work is distributed under GPLv2 too. The
embedded BSD-licensed assets below are GPL-compatible.

It **embeds** the following third-party assets via `//go:embed` (directory
`assets/`); redistributing this repo or a binary built from it redistributes
these, so their notices and source obligations are reproduced here.

---

## 1. `assets/packed_codebooks_aoTuV_603.bin` — BSD 3-Clause

The Vorbis codebook library, taken verbatim from **ww2ogg** by Adam Gashlin
(hcs). The codebooks themselves are the static codebooks of **aoTuV beta 6.03**
(Aoyumi), derived from the Xiph.org Vorbis reference.

- ww2ogg: https://github.com/hcs64/ww2ogg
- aoTuV:  https://ao-yumi.github.io/aotuv_web/

```
Copyright (c) 2002, Xiph.org Foundation
Copyright (c) 2009-2016, Adam Gashlin

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:
- Redistributions of source code must retain the above copyright notice, this
  list of conditions and the following disclaimer.
- Redistributions in binary form must reproduce the above copyright notice,
  this list of conditions and the following disclaimer in the documentation
  and/or other materials provided with the distribution.
- Neither the name of the Xiph.org Foundation nor the names of its contributors
  may be used to endorse or promote products derived from this software without
  specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS ``AS IS''
AND ANY EXPRESS OR IMPLIED WARRANTIES ... ARE DISCLAIMED. (full BSD-3 text)
```

---

## 2. `assets/oggenc2.exe` — GNU GPL v2 (IMPORTANT)

A prebuilt **OggEnc v2.88** for Windows from RareWares, statically linking
**vorbis-tools** with **libvorbis 1.3.7 + aoTuV beta 6.03 (Lancer Mod)**,
libogg, and libFLAC.

- vorbis-tools (the `oggenc` front-end) is licensed under the **GNU General
  Public License, version 2**. Because the binary as distributed includes
  vorbis-tools, **the `oggenc2.exe` binary as a whole is covered by GPLv2** —
  the same license as this project, so there is no compatibility question.
- Under GPLv2 §3, distributing `oggenc2.exe` (and this project's binaries that
  embed it) obligates you to make the corresponding source available. This
  project's own source is the repository; `oggenc2.exe`'s source is upstream:
  - vorbis-tools: https://gitlab.xiph.org/xiph/vorbis-tools
  - libvorbis:    https://gitlab.xiph.org/xiph/vorbis
  - aoTuV b6.03:  https://ao-yumi.github.io/aotuv_web/
  - RareWares build: https://www.rarewares.org/ogg-oggenc.php
- The full GPLv2 text: https://www.gnu.org/licenses/old-licenses/gpl-2.0.txt
  (include a copy as `COPYING.GPL2` when redistributing).

Underlying libvorbis/libogg/aoTuV are BSD-licensed (Xiph), but the combined
`oggenc2.exe` is governed by GPLv2 due to the vorbis-tools front-end.

---

## 3. `assets/libFLAC.dll` — BSD 3-Clause (Xiph.org)

The FLAC decoding library, used by `oggenc2.exe` to read `.flac` inputs.
libFLAC is licensed under the Xiph BSD 3-Clause license (the same terms as #1).

- FLAC: https://xiph.org/flac/

---

## Not bundled

**vgmstream** (https://github.com/vgmstream/vgmstream) and **ww2ogg** are used
only as development/validation oracles. They are **not** embedded or
redistributed by this package.
