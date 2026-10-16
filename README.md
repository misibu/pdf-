# PDF Color Checker 5.4

**Однофайловое приложение для Windows 64-bit:** проверяет PDF на RGB/CMYK, подсвечивает результаты и сохраняет TXT со списком проблемных файлов. Преобразует RGB PDF в CMYK по ISO Coated v2 (FOGRA39), используя Ghostscript, встроенный в EXE.

## Как собрать EXE
1. На главной странице репозитория нажмите **Add file → Upload files**.
2. Загрузите **PDF_Color_Checker_5.4_Build_Source.zip** без распаковки и нажмите **Commit changes**.
3. Перейдите во вкладку **Actions → Build PDF Color Checker 5.4 (portable)**. Дождитесь зелёной отметки и скачайте архив из блока **Artifacts**. Внутри будет **PDF_Color_Checker_5.4_Portable.exe**.

Сборка автоматически скачивает и проверяет Ghostscript, добавляет ICC-профиль FOGRA39 и запускает тест реальной PDF-конвертации. Отдельно устанавливать Ghostscript сотрудникам не понадобится.

**Важно:** конвертация не заменяет профессиональный Preflight. Код Ghostscript распространяется по AGPLv3 либо коммерческой лицензии Artifex; соблюдайте её условия.