<?php

use App\Support\Helper;

class DemoController extends BaseController
{
    public function run($input)
    {
        return Helper::exec($input);
    }
}
