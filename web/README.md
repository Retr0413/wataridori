# web

Wataridori の最小 Web UI。

Phase 2 の初期版では React / Next.js などのフレームワークを使わず、TypeScript と
ブラウザ標準の ES modules だけで構成する。`wataridori serve` がこのディレクトリの
assets を配信し、ブラウザは Connect RPC の JSON protocol で `DeploymentService` を呼び出す。

## 構成

- `src/api/`: Connect RPC JSON protocol の薄い client と response 型
- `src/features/`: Status / History / Promote Plan / Apply Dry Run の画面ロジック
- `src/ui/`: DOM helper、format、toast などの表示共通処理
- `dist/`: `npm run build` で生成され、Go バイナリに embed される配布用 JavaScript

## 開発

```sh
cd web
npm install
npm run build
```
