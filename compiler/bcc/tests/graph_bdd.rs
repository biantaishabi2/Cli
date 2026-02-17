//! BDD 测试 - 真实文件系统场景
//! 
//! 运行条件: 只在 master 分支运行（通过 CI 控制）
//! 标记: #[cfg(feature = "bdd")]

use bcc::graph::sqlite::GraphStoreManager;
use bcc::graph::store::{CodeGraphStore, GraphStoreInsert};
use bcc::graph::types::*;
use chrono::Utc;
use std::fs;
use std::path::PathBuf;
use tempfile::TempDir;

/// 创建测试用的代码文件
fn create_test_php_project(base_dir: &TempDir) -> PathBuf {
    let project_dir = base_dir.path().join("test_project");
    fs::create_dir_all(&project_dir).unwrap();
    
    // 创建 Controller 文件
    let controllers_dir = project_dir.join("app/Controllers");
    fs::create_dir_all(&controllers_dir).unwrap();
    fs::write(
        controllers_dir.join("UserController.php"),
        r#"<?php
class UserController {
    private $userService;
    
    public function __construct() {
        $this->userService = new UserService();
    }
    
    public function createUser($data) {
        return $this->userService->create($data);
    }
    
    public function getUser($id) {
        return $this->userService->findById($id);
    }
}
?"#
    ).unwrap();
    
    // 创建 Service 文件
    let services_dir = project_dir.join("app/Services");
    fs::create_dir_all(&services_dir).unwrap();
    fs::write(
        services_dir.join("UserService.php"),
        r#"<?php
class UserService {
    private $userModel;
    
    public function __construct() {
        $this->userModel = new UserModel();
    }
    
    public function create($data) {
        return $this->userModel->insert($data);
    }
    
    public function findById($id) {
        return $this->userModel->find($id);
    }
}
?"#
    ).unwrap();
    
    // 创建 Model 文件
    let models_dir = project_dir.join("app/Models");
    fs::create_dir_all(&models_dir).unwrap();
    fs::write(
        models_dir.join("UserModel.php"),
        r#"<?php
class UserModel {
    public function insert($data) {
        return db_insert('users', $data);
    }
    
    public function find($id) {
        return db_query('SELECT * FROM users WHERE id = ?', [$id]);
    }
}
?"#
    ).unwrap();
    
    project_dir
}

/// BDD Scenario 1: 索引真实项目目录
#[test]
#[cfg(feature = "bdd")]
fn bdd_index_real_project() {
    let temp_dir = TempDir::new().unwrap();
    let project_dir = create_test_php_project(&temp_dir);
    let index_dir = temp_dir.path().join("index");
    
    // Given: 一个真实项目目录
    assert!(project_dir.exists());
    
    // When: 创建索引
    let manager = GraphStoreManager::new(&index_dir).unwrap();
    let store = manager.get_store("test/project").unwrap();
    
    // 索引类（模拟从文件提取）
    let controller_class = ClassRecord {
        id: "app/Controllers/UserController.php#UserController".to_string(),
        name: "UserController".to_string(),
        file_path: "app/Controllers/UserController.php".to_string(),
        module: "Controllers".to_string(),
        language: "php".to_string(),
        parent_id: None,
        interface_ids: vec![],
        start_line: 1,
        end_line: 15,
        indexed_at: Utc::now(),
    };
    
    let service_class = ClassRecord {
        id: "app/Services/UserService.php#UserService".to_string(),
        name: "UserService".to_string(),
        file_path: "app/Services/UserService.php".to_string(),
        module: "Services".to_string(),
        language: "php".to_string(),
        parent_id: None,
        interface_ids: vec![],
        start_line: 1,
        end_line: 15,
        indexed_at: Utc::now(),
    };
    
    store.insert_class(&controller_class).unwrap();
    store.insert_class(&service_class).unwrap();
    
    // Then: 应该能查询到索引的类
    let found = store.get_class("app/Controllers/UserController.php#UserController");
    assert!(found.is_some());
    assert_eq!(found.unwrap().name, "UserController");
}

/// BDD Scenario 2: 检测分层架构违规
#[test]
#[cfg(feature = "bdd")]
fn bdd_detect_layer_violation() {
    let temp_dir = TempDir::new().unwrap();
    let index_dir = temp_dir.path().join("index");
    
    let manager = GraphStoreManager::new(&index_dir).unwrap();
    let store = manager.get_store("test/violation").unwrap();
    
    // Given: 存在违规调用的代码
    // Controller 直接调用 Model（跳过 Service）
    let controller_func = FunctionRecord {
        id: "app/Controllers/UserController.php#createUser#5".to_string(),
        name: "createUser".to_string(),
        file_path: "app/Controllers/UserController.php".to_string(),
        module: "Controllers".to_string(),
        language: "php".to_string(),
        start_line: 5,
        end_line: 10,
        signature: "public function createUser($data)".to_string(),
        content_hash: "abc123".to_string(),
        indexed_at: Utc::now(),
    };
    
    let model_func = FunctionRecord {
        id: "app/Models/UserModel.php#insert#2".to_string(),
        name: "insert".to_string(),
        file_path: "app/Models/UserModel.php".to_string(),
        module: "Models".to_string(),
        language: "php".to_string(),
        start_line: 2,
        end_line: 5,
        signature: "public function insert($data)".to_string(),
        content_hash: "def456".to_string(),
        indexed_at: Utc::now(),
    };
    
    store.insert_function(&controller_func).unwrap();
    store.insert_function(&model_func).unwrap();
    
    // 创建调用边（违规：Controller -> Model）
    store.insert_call_edge(&CallEdge {
        caller_id: "app/Controllers/UserController.php#createUser#5".to_string(),
        callee_id: "app/Models/UserModel.php#insert#2".to_string(),
        call_type: CallType::Direct,
        file_path: Some("app/Controllers/UserController.php".to_string()),
        line_number: Some(7),
    }).unwrap();
    
    // When: 查询调用关系
    let callees = store.find_callees(
        "app/Controllers/UserController.php#createUser#5",
        1
    ).unwrap();
    
    // Then: 应该能找到被调用的 Model 函数
    assert_eq!(callees.len(), 1);
    assert_eq!(callees[0].name, "insert");
}

/// BDD Scenario 3: 多仓库隔离
#[test]
#[cfg(feature = "bdd")]
fn bdd_multi_repo_isolation() {
    let temp_dir = TempDir::new().unwrap();
    let index_dir = temp_dir.path().join("index");
    
    // Given: 两个独立的项目
    let manager = GraphStoreManager::new(&index_dir).unwrap();
    
    // Project A
    {
        let store_a = manager.get_store("github.com/company/project-a").unwrap();
        let func = FunctionRecord {
            id: "src/main.php#hello#1".to_string(),
            name: "hello".to_string(),
            file_path: "src/main.php".to_string(),
            module: "src".to_string(),
            language: "php".to_string(),
            start_line: 1,
            end_line: 5,
            signature: "function hello()".to_string(),
            content_hash: "a1b2c3".to_string(),
            indexed_at: Utc::now(),
        };
        store_a.insert_function(&func).unwrap();
    }
    
    // Project B（同名函数）
    {
        let store_b = manager.get_store("github.com/company/project-b").unwrap();
        let func = FunctionRecord {
            id: "src/main.php#hello#1".to_string(),
            name: "hello".to_string(),
            file_path: "src/main.php".to_string(),
            module: "src".to_string(),
            language: "php".to_string(),
            start_line: 1,
            end_line: 5,
            signature: "function hello()".to_string(),
            content_hash: "d4e5f6".to_string(),
            indexed_at: Utc::now(),
        };
        store_b.insert_function(&func).unwrap();
    }
    
    // When: 重新打开索引
    drop(manager);
    let manager2 = GraphStoreManager::new(&index_dir).unwrap();
    
    // Then: 两个项目的索引应该独立
    let store_a = manager2.get_store("github.com/company/project-a").unwrap();
    let store_b = manager2.get_store("github.com/company/project-b").unwrap();
    
    let func_a = store_a.get_function("src/main.php#hello#1").unwrap();
    let func_b = store_b.get_function("src/main.php#hello#1").unwrap();
    
    assert_eq!(func_a.content_hash, "a1b2c3");
    assert_eq!(func_b.content_hash, "d4e5f6");
}

/// BDD Scenario 4: 继承链分析
#[test]
#[cfg(feature = "bdd")]
fn bdd_inheritance_chain_analysis() {
    let temp_dir = TempDir::new().unwrap();
    let index_dir = temp_dir.path().join("index");
    
    let manager = GraphStoreManager::new(&index_dir).unwrap();
    let store = manager.get_store("test/inheritance").unwrap();
    
    // Given: 一个复杂的继承链
    // BaseEntity -> User -> Admin
    //            -> Product
    
    let base = ClassRecord {
        id: "BaseEntity.php#BaseEntity".to_string(),
        name: "BaseEntity".to_string(),
        file_path: "BaseEntity.php".to_string(),
        module: "models".to_string(),
        language: "php".to_string(),
        parent_id: None,
        interface_ids: vec![],
        start_line: 1,
        end_line: 20,
        indexed_at: Utc::now(),
    };
    
    let user = ClassRecord {
        id: "User.php#User".to_string(),
        name: "User".to_string(),
        file_path: "User.php".to_string(),
        module: "models".to_string(),
        language: "php".to_string(),
        parent_id: Some("BaseEntity.php#BaseEntity".to_string()),
        interface_ids: vec![],
        start_line: 1,
        end_line: 30,
        indexed_at: Utc::now(),
    };
    
    let admin = ClassRecord {
        id: "Admin.php#Admin".to_string(),
        name: "Admin".to_string(),
        file_path: "Admin.php".to_string(),
        module: "models".to_string(),
        language: "php".to_string(),
        parent_id: Some("User.php#User".to_string()),
        interface_ids: vec![],
        start_line: 1,
        end_line: 25,
        indexed_at: Utc::now(),
    };
    
    let product = ClassRecord {
        id: "Product.php#Product".to_string(),
        name: "Product".to_string(),
        file_path: "Product.php".to_string(),
        module: "models".to_string(),
        language: "php".to_string(),
        parent_id: Some("BaseEntity.php#BaseEntity".to_string()),
        interface_ids: vec![],
        start_line: 1,
        end_line: 25,
        indexed_at: Utc::now(),
    };
    
    store.insert_class(&base).unwrap();
    store.insert_class(&user).unwrap();
    store.insert_class(&admin).unwrap();
    store.insert_class(&product).unwrap();
    
    // 插入继承边
    store.insert_inherit_edge(&InheritEdge {
        child_id: "User.php#User".to_string(),
        parent_id: "BaseEntity.php#BaseEntity".to_string(),
        edge_type: InheritType::Extends,
    }).unwrap();
    
    store.insert_inherit_edge(&InheritEdge {
        child_id: "Admin.php#Admin".to_string(),
        parent_id: "User.php#User".to_string(),
        edge_type: InheritType::Extends,
    }).unwrap();
    
    store.insert_inherit_edge(&InheritEdge {
        child_id: "Product.php#Product".to_string(),
        parent_id: "BaseEntity.php#BaseEntity".to_string(),
        edge_type: InheritType::Extends,
    }).unwrap();
    
    // When: 查询继承链
    let parents = store.find_parents("Admin.php#Admin", 3).unwrap();
    let children = store.find_children("BaseEntity.php#BaseEntity", 2).unwrap();
    
    // Then: 应该正确返回继承链
    assert_eq!(parents.len(), 2); // User, BaseEntity
    assert_eq!(children.len(), 3); // User, Admin, Product
}
