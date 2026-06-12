# go-musthave-shortener-tpl

Шаблон репозитория для трека «Сервис сокращения URL».

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без префикса `https://`) для создания модуля.

## Обновление шаблона

Чтобы иметь возможность получать обновления автотестов и других частей шаблона, выполните команду:

```
git remote add -m v2 template https://github.com/Yandex-Practicum/go-musthave-shortener-tpl.git
```

Для обновления кода автотестов выполните команду:

```
git fetch template && git checkout template/v2 .github
```

Затем добавьте полученные изменения в свой репозиторий.

## Запуск автотестов

Для успешного запуска автотестов называйте ветки `iter<number>`, где `<number>` — порядковый номер инкремента. Например, в ветке с названием `iter4` запустятся автотесты для инкрементов с первого по четвёртый.

При мёрже ветки с инкрементом в основную ветку `main` будут запускаться все автотесты.

Подробнее про локальный и автоматический запуск читайте в [README автотестов](https://github.com/Yandex-Practicum/go-autotests).

## Сборка

```powershell
go build -ldflags ('-X main.buildVersion=1.0.0 -X main.buildCommit=dev -X main.buildDate=' + (Get-Date -Format 'yyyy/MM/dd_HH:mm:ss')) -o shortener.exe .\cmd\shortener\
```

## Структура проекта

Приведённая в этом репозитории структура проекта является рекомендуемой, но не обязательной.

Это лишь пример организации кода, который поможет вам в реализации сервиса.

При необходимости можно вносить изменения в структуру проекта, использовать любые библиотеки и предпочитаемые структурные паттерны организации кода приложения, например:
- **DDD** (Domain-Driven Design)
- **Clean Architecture**
- **Hexagonal Architecture**
- **Layered Architecture**

---

## Профилирование и оптимизация памяти

Для анализа аллокаций памяти использовался `pprof` с нагрузочным тестированием через `hey` (30 000 запросов, 50 воркеров).

### Сравнение профилей (base vs result)

```
go tool pprof -top -nodecount=20 -diff_base=profiles/base.pprof -alloc_space profiles/result.pprof

File: shortener.exe
Type: alloc_space
Time: 2026-05-10 21:26:14 +07
Showing nodes accounting for 20.50MB, 6.97% of 293.98MB total
Dropped 4 nodes (cum <= 1.47MB)
Showing top 20 nodes out of 175
      flat  flat%   sum%        cum   cum%
    8.50MB  2.89%  2.89%     8.50MB  2.89%  net/url.parse
       7MB  2.38%  5.27%        7MB  2.38%  net/http.(*Request).WithContext
   -6.50MB  2.21%  3.06%    -6.50MB  2.21%  github.com/golang-jwt/jwt/v5.NewWithClaims (inline)
    6.50MB  2.21%  5.27%       12MB  4.08%  net/http.(*conn).readRequest
       5MB  1.70%  6.97%        5MB  1.70%  net/http.Header.Clone (inline)
   -4.50MB  1.53%  5.44%    -2.50MB  0.85%  crypto/internal/fips140/hmac.New
    3.50MB  1.19%  6.63%     3.50MB  1.19%  encoding/base64.(*Encoding).EncodeToString
   -3.01MB  1.02%  5.61%    -3.01MB  1.02%  sync.(*Pool).pinSlow
       2MB  0.68%  6.29%        2MB  0.68%  internal/bytealg.MakeNoZero
      -2MB  0.68%  5.61%    -2.50MB  0.85%  io.ReadAll
       2MB  0.68%  6.29%        2MB  0.68%  crypto/internal/fips140/sha256.New (inline)
       2MB  0.68%  6.97%        2MB  0.68%  sync.(*poolChain).pushHead
      -2MB  0.68%  6.29%       -2MB  0.68%  github.com/go-chi/chi/v5/middleware.NewWrapResponseWriter
       2MB  0.68%  6.97%        2MB  0.68%  net/http.MaxBytesReader
   -1.50MB  0.51%  6.46%    -5.50MB  1.87%  github.com/golang-jwt/jwt/v5.(*Token).SignedString
    1.50MB  0.51%  6.97%    -0.50MB  0.17%  go-url-shortener/internal/server.NewRouter.Logger.func1.1
    1.50MB  0.51%  7.48%     0.50MB  0.17%  net/url.(*URL).joinPath
   -1.50MB  0.51%  6.97%    -4.50MB  1.53%  github.com/golang-jwt/jwt/v5.(*SigningMethodHMAC).Sign
   -1.50MB  0.51%  6.46%    -3.50MB  1.19%  github.com/go-chi/chi/v5/middleware.(*Compressor).Handler-fm.(*Compressor).Handler.func1
    1.50MB  0.51%  6.97%     1.50MB  0.51%  go.uber.org/zap/zapcore.init.func4
```

### Основные улучшения

| Компонент | Экономия | Оптимизация |
|---|---|---|
| `jwt.NewWithClaims` | **-6.50 MB** | JWT-токены создаются реже (кэширование)
| `hmac.New` | **-4.50 MB** | Меньше HMAC-операций при подписи
| `(*Token).SignedString` | **-5.50 MB** (cum) | Сокращение цепочки аллокаций при подписи
| `io.ReadAll` | **-2.00 MB** | Замена на `MaxBytesReader` + `io.NopCloser`
| `URLEntry` pointer→value | **-2.00 MB** | Хранение `URLEntry` как значения (`URLEntry{}`) в map вместо указателя (`*URLEntry`) — 0 аллокаций на элемент вместо 1
| `(*SigningMethodHMAC).Sign` | **-4.50 MB** (cum) | Меньше вызовов HMAC-подписи

### Ключевые выводы

1. **JWT-подпись — основной потребитель памяти** (24.33% всех аллокаций, ~71.5 MB).
   Каждый запрос без cookie создаёт новый токен. В production с постоянными клиентами эффект меньше.

2. **`io.ReadAll`** (5.79%, ~17 MB) — заменён на `http.MaxBytesReader` + `io.NopCloser`,
   что дало экономию **-2 MB**.

3. **`GenerateShortID`**: `[]rune` → `[]byte` снизило аллокации с 16→0 B/op и
   ускорило функцию на **34%** (97.78 → 64.13 ns/op).
