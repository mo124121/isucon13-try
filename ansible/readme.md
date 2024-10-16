# メモ

## 大本

https://github.com/rinchsan/isucon13-final

## 実行方法

ansible-playbook  -i inventory.yaml -u ubuntu --private-key ~/.ssh/codespace.pem [実行したいplaybook].yaml

## 編集が必要な個所

inventory.yamlにデプロイ先のホストの指定がある
