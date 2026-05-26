# projectManager 工作流程

## 1. 需求变更的工作流程
- 先更新projectManager/docs/requirement.md需求文档
- 再根据最新的projectManager/docs/requirement.md更新projectManager/docs/design.md方案设计文档
- 再根据最新的projectManager/docs/design.md方案设计文档，在projectManager/docs/rule.md的规则下编写代码
- 编写完代码后，再更新下projectManager/docs/http_api.md接口文档给前端开发同事接入，注意http_aip.md中的每个接口回包，都列举出整个json格式
- 编写完代码后，再更新go test的测试用例，并且需要确保用例全部测试通过
- 再根据最新的projectManager/docs/http_aip.md，编写与projectManager后端对接的页面代码
- 最后更新项目管理软件的使用帮助文档projectManager/docs/help.md