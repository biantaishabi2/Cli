//! SQLite 存储测试

#[cfg(test)]
mod tests {
    use super::super::*;
    use chrono::Utc;

    fn create_test_function(id: &str, name: &str, module: &str) -> FunctionRecord {
        FunctionRecord {
            id: id.to_string(),
            name: name.to_string(),
            file_path: format!("src/{}.php", module),
            module: module.to_string(),
            language: "php".to_string(),
            start_line: 10,
            end_line: 20,
            signature: format!("function {}()", name),
            content_hash: "abc123".to_string(),
            indexed_at: Utc::now(),
        }
    }

    #[test]
    fn test_sqlite_store_creation() {
        let store = SqliteGraphStore::new_in_memory().unwrap();
        // 能创建成功即通过
        let _ = store;
    }

    #[test]
    fn test_function_crud() {
        let store = SqliteGraphStore::new_in_memory().unwrap();
        
        let func = create_test_function("test.php#foo#10", "foo", "test");
        store.insert_function(&func).unwrap();
        
        let found = store.get_function("test.php#foo#10");
        assert!(found.is_some());
        assert_eq!(found.unwrap().name, "foo");
    }

    #[test]
    fn test_find_by_module() {
        let store = SqliteGraphStore::new_in_memory().unwrap();
        
        let func1 = create_test_function("order.php#create#10", "create", "order");
        let func2 = create_test_function("order.php#update#20", "update", "order");
        let func3 = create_test_function("user.php#login#10", "login", "user");
        
        store.insert_function(&func1).unwrap();
        store.insert_function(&func2).unwrap();
        store.insert_function(&func3).unwrap();
        
        let order_funcs = store.find_by_module("order");
        assert_eq!(order_funcs.len(), 2);
        
        let user_funcs = store.find_by_module("user");
        assert_eq!(user_funcs.len(), 1);
    }

    #[test]
    fn test_call_edge() {
        let store = SqliteGraphStore::new_in_memory().unwrap();
        
        // 插入两个函数
        let caller = create_test_function("a.php#caller#10", "caller", "a");
        let callee = create_test_function("b.php#callee#10", "callee", "b");
        store.insert_function(&caller).unwrap();
        store.insert_function(&callee).unwrap();
        
        // 插入调用关系
        let edge = CallEdge {
            caller_id: caller.id.clone(),
            callee_id: callee.id.clone(),
            call_type: CallType::Direct,
            file_path: Some("a.php".to_string()),
            line_number: Some(15),
        };
        store.insert_call_edge(&edge).unwrap();
        
        // 查询调用者
        let callers = store.find_callers(&callee.id, 1);
        assert_eq!(callers.len(), 1);
        assert_eq!(callers[0].name, "caller");
        
        // 查询被调用者
        let callees = store.find_callees(&caller.id, 1);
        assert_eq!(callees.len(), 1);
        assert_eq!(callees[0].name, "callee");
    }

    #[test]
    fn test_analyze_impact() {
        let store = SqliteGraphStore::new_in_memory().unwrap();
        
        // 创建调用链: a -> b -> c
        let func_a = create_test_function("a.php#a#10", "a", "a");
        let func_b = create_test_function("b.php#b#10", "b", "b");
        let func_c = create_test_function("c.php#c#10", "c", "c");
        
        store.insert_function(&func_a).unwrap();
        store.insert_function(&func_b).unwrap();
        store.insert_function(&func_c).unwrap();
        
        // a -> b
        store.insert_call_edge(&CallEdge {
            caller_id: func_a.id.clone(),
            callee_id: func_b.id.clone(),
            call_type: CallType::Direct,
            file_path: None,
            line_number: None,
        }).unwrap();
        
        // b -> c
        store.insert_call_edge(&CallEdge {
            caller_id: func_b.id.clone(),
            callee_id: func_c.id.clone(),
            call_type: CallType::Direct,
            file_path: None,
            line_number: None,
        }).unwrap();
        
        // 分析 b 的影响
        let impact = store.analyze_impact(&[func_b.id.clone()]);
        
        // b 是直接影响
        assert_eq!(impact.direct_changes.len(), 1);
        
        // a 是上游（调用 b）
        assert_eq!(impact.upstream_impact.len(), 1);
        assert_eq!(impact.upstream_impact[0].function.name, "a");
        
        // c 是下游（被 b 调用）
        assert_eq!(impact.downstream_impact.len(), 1);
        assert_eq!(impact.downstream_impact[0].function.name, "c");
    }
}
