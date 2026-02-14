<?php

use Carbon\Carbon;

class OauserController extends ObaseController
{
    use MsbcArticleTrait;

    protected $isInnerUser = false;

    public function initialize()
    {
        parent::initialize();
        $this->userId = $this->loginer()['uuid'];
    }

    /**
     * @desc 获取关注类型信息
     */
    public function attentionnewsAction()
    {
        $user = isset($_GET['user']) ? trim(strval($_GET['user'])) : '';
        $data = SysMsg::F(" `rcate` = '{$attention}' AND `receiver` = '{$user}' ");
        return $this->_jsonformat($data);
    }

    /**
     * @desc 评论操作
     */
    public function commentAction()
    {
        $content = $_POST['content'] ?? '';
        $rtn = [];
        $rtn["content"] = htmlspecialchars($content, ENT_QUOTES, 'UTF-8');
        OaComment::create($rtn);
        return $this->successWithMessage('操作成功');
    }

    private function _formatUser($user)
    {
        return [
            'name' => SysUser::usrname($user),
            'img'  => SysFile::userAvatar($user, true, true),
        ];
    }
}
