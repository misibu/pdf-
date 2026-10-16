# PDF Color Checker

Приложение для проверки RGB/CMYK в PDF и преобразования RGB в CMYK с ICC-профилем ISO Coated v2 (FOGRA39).

## Сборка Windows EXE

Репозиторий будет собираться через GitHub Actions. Сборка скачивает официальный Ghostscript, проверяет SHA-256, добавляет ICC FOGRA39, встраивает движок в один EXE и проверяет конвертацию на тестовом PDF.

Для внутреннего использования в типографии. Учитывайте условия AGPLv3 для Ghostscript и лицензию ICC-профиля ECI.
