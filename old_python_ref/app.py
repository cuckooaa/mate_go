from urllib.parse import urlparse
from flask import Flask, render_template
from flask_sqlalchemy import SQLAlchemy
from datetime import datetime, timedelta
import os
import uuid
import math
import pytz
from sqlalchemy import JSON, desc, func
from werkzeug.security import generate_password_hash, check_password_hash
from werkzeug.utils import secure_filename
from flask import send_from_directory
from flask import request, jsonify
from flask import session, redirect, url_for
# from PIL import Image # <-- 导入 PIL 库
# from io import BytesIO # <-- 用于处理文件流

# 创建 Flask 应用实例
app = Flask(__name__, instance_relative_config=True)

# 配置文件路径，以便在instance文件夹下找到
app.config.from_mapping(
    SECRET_KEY='Sleepwellandstaysafe', # 开发秘钥，正式环境需要替换
    DATABASE=os.path.join(app.instance_path, 'database.db'),
    SQLALCHEMY_DATABASE_URI=f"sqlite:///{os.path.join(app.instance_path, 'database.db')}"
)

# 配置上传文件夹的路径
UPLOAD_FOLDER = os.path.join(app.instance_path, 'uploads')
app.config['UPLOAD_FOLDER'] = UPLOAD_FOLDER

app.config['SESSION_PERMANENT'] = True
# 一周有 7 天 * 24 小时 * 60 分钟 * 60 秒 = 604800 秒
# Flask 使用 timedelta 来设置时间
app.config['PERMANENT_SESSION_LIFETIME'] = timedelta(weeks=1)

app.config['MAX_CONTENT_LENGTH'] = 20 * 1024 * 1024  # 允许最大 20MB

# 确保上传文件夹存在
os.makedirs(UPLOAD_FOLDER, exist_ok=True)

# 确保实例文件夹存在
os.makedirs(app.instance_path, exist_ok=True)

# 初始化 SQLAlchemy
db = SQLAlchemy(app)

# 定义数据库模型
# 这些模型类将映射到数据库表
class User(db.Model):
    __tablename__ = 'users'
    id = db.Column(db.String, primary_key=True)
    email = db.Column(db.String(120), unique=True, nullable=False)
    password = db.Column(db.String(120), nullable=False)
    current_points = db.Column(db.Float, default=0)
    total_earned_points = db.Column(db.Float, default=0)
    nickname = db.Column(db.String(80), nullable=True)
    avatar = db.Column(db.String(255), nullable=True)
    last_settlement_date = db.Column(db.DateTime, default=datetime.utcnow)

    exchange_rate = db.Column(db.Integer, default=10)

class Task(db.Model):
    __tablename__ = 'tasks'
    id = db.Column(db.Integer, primary_key=True, autoincrement=True)
    user_id = db.Column(db.String, db.ForeignKey('users.id'))
    description = db.Column(db.String(255), nullable=False)
    type = db.Column(db.String(20), nullable=False)
    points = db.Column(db.Float, nullable=False)
    status = db.Column(db.String(20), default='pending')
    created_at = db.Column(db.DateTime, default=datetime.utcnow)
    time_spent_seconds = db.Column(db.Integer, default=0)
    timer_start_time = db.Column(db.Float, nullable=True)# nullable=True表示可以为空，计时停止时为空

class WeekRecord(db.Model):
    __tablename__ = 'week_record'
    id = db.Column(db.Integer, primary_key=True, autoincrement=True)
    user_id = db.Column(db.String, db.ForeignKey('users.id'))
    name = db.Column(db.String(100), nullable=False)
    total_num = db.Column(db.Integer, nullable=False)
    total_points = db.Column(db.Float, nullable=False)
    created_at = db.Column(db.DateTime, default=datetime.utcnow)
    total_time=db.Column(db.Integer, default=0)
    task_lists = db.Column(JSON, nullable=True)

class ShopItem(db.Model):
    __tablename__ = 'shop_items'
    id = db.Column(db.Integer, primary_key=True, autoincrement=True)
    name = db.Column(db.String(100), nullable=False)
    points = db.Column(db.Float, nullable=False)
    image = db.Column(db.String(255), nullable=True)
    type = db.Column(db.String(20), nullable=False)
    user_id = db.Column(db.String, db.ForeignKey('users.id'), nullable=True)
    created_at = db.Column(db.DateTime, default=datetime.utcnow)

class RedeemedItem(db.Model):
    __tablename__ = 'redeemed_items'
    id = db.Column(db.Integer, primary_key=True, autoincrement=True)
    user_id = db.Column(db.String, db.ForeignKey('users.id'))
    item_name = db.Column(db.String(100), nullable=False)
    item_points = db.Column(db.Float, nullable=False)
    redeemed_at = db.Column(db.DateTime, default=datetime.utcnow)
    item_image = db.Column(db.String(255),nullable=True)

# 路由和主页函数保持不变
@app.route('/')
def home():
    return render_template('index.html')

# 新增一个命令行函数来初始化数据库
@app.cli.command("init-db")
def init_db_command():
    """Clear existing data and create new tables."""
    with app.app_context():
        db.drop_all()
        db.create_all()
    print('Initialized the database.')


# app.py

# ... (在 index 路由下方)

@app.route('/weekrecord')
def weekrecord():
    week_record=request.args.get('week_record', 0)
    return render_template('weekrecord.html',week_record=week_record)


@app.route('/weekrecord/<int:weekrecord_id>')
def get_task_lists(weekrecord_id):
    weekrecord=WeekRecord.query.filter_by(id=weekrecord_id).first()
    # 将查询结果转换为可序列化的字典列表
    task_lists=weekrecord.task_lists
    tasks = []
    for task_id in task_lists:
        task=Task.query.filter_by(id=task_id).first()
        LOCAL_TIMEZONE = pytz.timezone('Asia/Shanghai')
        utc_time_naive = task.created_at
        utc_time_aware = pytz.utc.localize(utc_time_naive)
        local_time = utc_time_aware.astimezone(LOCAL_TIMEZONE)
        tasks.append({
            'id': task.id,
            'description': task.description,
            'type': task.type,
            'points': task.points,
            'status': task.status,
            'created_at': local_time.strftime('%Y-%m-%d') ,
            'time_spent_seconds':task.time_spent_seconds,
            'timer_start_time':task.timer_start_time
        })
    tasks = sorted(tasks, key=lambda task: task['created_at'], reverse=True)
    result={
        'tasks':tasks,
        'name':weekrecord.name
    }

    return jsonify(result), 200

# app.py

@app.route('/rankinglist')
def rankinglist():
    return render_template('rankinglist.html')

@app.route('/api/leaderboard', methods=['GET'])
def get_leaderboard():
    leaderboard_data = db.session.query(
        User.nickname,
        User.avatar, # <-- 额外查询 User.avatar
        func.sum(Task.points).label('total_points')
    ).join(Task, User.id == Task.user_id) \
     .filter(Task.status == 'completed') \
     .group_by(User.nickname, User.avatar) \
     .order_by(desc('total_points')) \
     .all()

    # 格式化结果为列表，每个元素是包含用户信息的字典
    leaderboard_list = [] # 将字典改为列表，因为需要存储更多信息

    # 遍历查询结果，结果是 (nickname, avatar, total_points) 元组
    for nickname, avatar, total_points in leaderboard_data:
        leaderboard_list.append({
            'nickname': nickname,
            'avatar': avatar, # <-- 包含头像 URL
            'total_points': float(total_points) # 确保转换为 float
        })

    # 返回 JSON 响应
    # 格式示例: [ {"nickname": "用户A", "avatar": "/uploads/a.png", "total_points": 150.5}, ... ]
    return jsonify(leaderboard_list), 200


# 注册 API
@app.route('/api/register', methods=['POST'])
def register():
    data = request.get_json()
    email = data.get('email')
    password = data.get('password')
    nickname = data.get('nickname') or '用户' + str(uuid.uuid4())[:4]

    if not email or not password:
        return jsonify({'error': '邮箱和密码是必填项'}), 400

    # 检查邮箱是否已注册
    user = User.query.filter_by(email=email).first()
    if user:
        return jsonify({'error': '该邮箱已被注册'}), 409

    # 创建新用户
    hashed_password = generate_password_hash(password, method='pbkdf2:sha256')
    new_user = User(
        id=str(uuid.uuid4()),
        email=email,
        password=hashed_password,
        nickname=nickname,
        avatar="/static/a2.png"
    )

    db.session.add(new_user)
    db.session.commit()

    return jsonify({
        'message': '注册成功',
        'user_id': new_user.id
    }), 201

@app.route('/api/logout', methods=['POST'])
def logout():
    # 彻底清除会话中的所有内容
    session.clear()
    
    # 或者只清除你用来标记登录状态的键（更推荐彻底清除）
    # session.pop('user_id', None) 
    
    return jsonify({'success': True, 'message': 'Logged out successfully'}), 200

# 登录 API
@app.route('/api/login', methods=['POST'])
def login():
    data = request.get_json()
    email = data.get('email')
    password = data.get('password')

    if not email or not password:
        return jsonify({'error': '邮箱和密码是必填项'}), 400

    user = User.query.filter_by(email=email).first()
    if not user or not check_password_hash(user.password, password):
        return jsonify({'error': '邮箱或密码不正确'}), 401
    
    session['user_id'] = user.id
    session.permanent = True  # 设置会话永久性（可选，延长过期时间）

    return jsonify({
        'message': '登录成功',
        'user_id': user.id
    })

# app.py
@app.route('/api/check_login_status')
def check_login_status():
    if 'user_id' in session:
        # 用户已登录，返回用户的部分信息（比如昵称）
        user = User.query.get(session['user_id'])
        return jsonify({'is_logged_in': True, 'user_id':user.id}), 200
    else:
        # 用户未登录
        return jsonify({'is_logged_in': False}), 200


# 添加任务 API
@app.route('/api/tasks', methods=['POST'])
def create_task():
    # 假设用户 ID 是通过请求头或 body 传递的，这里我们使用简单的 body 传递
    data = request.get_json()
    user_id = data.get('user_id')
    description = data.get('description')
    task_type = data.get('type') # 's-u-c', 'ns-u-c' 等

    if not all([user_id, description, task_type]):
        return jsonify({'error': '缺少必要的任务信息'}), 400

    # 根据任务类型计算积分
    points_map = {
        's-u-c': 5,
        'ns-u-c': 3,
        's-nu-c': 4,
        's-u-nc': 4,
        'ns-nu-c': 3,
        'ns-u-nc': 2,
        's-nu-nc': 3,
        'ns-nu-nc': 1,
    }
    points = points_map.get(task_type, 0)

    # 创建新的 Task 对象并保存到数据库
    new_task = Task(
        user_id=user_id,
        description=description,
        type=task_type,
        points=points,
        status='pending'
    )
    
    db.session.add(new_task)
    db.session.commit()

    return jsonify({
        'message': '任务创建成功',
        'task': {
            'id': new_task.id,
            'description': new_task.description,
            'type': new_task.type,
            'points': new_task.points,
            'status': new_task.status
        }
    }), 201

# 获取任务列表 API
@app.route('/api/tasks/<string:user_id>', methods=['GET'])
def get_tasks(user_id):
    # 查询数据库中属于该用户的所有任务
    tasks = Task.query.filter_by(user_id=user_id).order_by(Task.created_at.desc()).all()

    week_records = WeekRecord.query.filter_by(user_id=user_id).order_by(WeekRecord.created_at.desc()).all()
    week_record = []
    for item in week_records:
        LOCAL_TIMEZONE = pytz.timezone('Asia/Shanghai')
        utc_time_naive = item.created_at
        utc_time_aware = pytz.utc.localize(utc_time_naive)
        local_time = utc_time_aware.astimezone(LOCAL_TIMEZONE)
        week_record.append({
            'id':item.id,
            'name': item.name,
            'total_points': item.total_points,
            'total_num': item.total_num,
            'created_at': local_time.strftime('%Y-%m-%d') ,
            'total_time':item.total_time
        })

    # 将查询结果转换为可序列化的字典列表
    tasks_list = []
    for task in tasks:
        LOCAL_TIMEZONE = pytz.timezone('Asia/Shanghai')
        utc_time_naive = task.created_at
        utc_time_aware = pytz.utc.localize(utc_time_naive)
        local_time = utc_time_aware.astimezone(LOCAL_TIMEZONE)
        tasks_list.append({
            'id': task.id,
            'description': task.description,
            'type': task.type,
            'points': task.points,
            'status': task.status,
            'created_at': local_time.strftime('%Y-%m-%d') ,
            'time_spent_seconds':task.time_spent_seconds,
            'timer_start_time':task.timer_start_time
        })

    result={
        'tasks_list':tasks_list,
        'week_record':week_record
    }
    
    return jsonify(result), 200

# 任务完成状态更新 API
@app.route('/api/tasks/<int:task_id>/complete', methods=['POST'])
def complete_task(task_id):
    data = request.get_json()
    user_id = data.get('user_id')

    # 查找任务
    task = Task.query.filter_by(id=task_id, user_id=user_id).first()
    if not task:
        return jsonify({'error': '任务不存在或不属于该用户'}), 404
    
    if task.time_spent_seconds<600:
        return jsonify({'error': '请确保任务计时达到10分钟以上，再点击完成'}), 404
    
    points=(task.points*0.5+1.5)*0.005*(task.time_spent_seconds/60)
    points=math.floor(points * 10) / 10

    # 更新任务状态
    task.status = 'completed'
    task.points = points

    # 更新用户积分
    user = User.query.filter_by(id=user_id).first()
    if user:
        user.current_points += points
        user.total_earned_points += points
    
    user.current_points=math.floor(user.current_points * 10) / 10
    user.total_earned_points=math.floor(user.total_earned_points * 10) / 10

    db.session.commit()

    return jsonify({'message': '任务已完成', 
                    'points': points,
                    'time_spent_seconds':task.time_spent_seconds}), 200

@app.route('/api/tasks/<int:task_id>/delete', methods=['DELETE'])
def delete_task(task_id):
    # 任务删除 API
    data = request.get_json()
    user_id = data.get('user_id')

    # 查找任务
    task = Task.query.filter_by(id=task_id, user_id=user_id).first()
    if not task:
        return jsonify({'error': '任务不存在或不属于该用户'}), 404

    # 删除任务
    db.session.delete(task)
    db.session.commit()

    return jsonify({'message': '任务已成功删除'}), 200



# app.py

# ... 其他路由 ...

# 任务计时数据同步 API (更新为同时处理开始时间)
@app.route('/api/tasks/<int:task_id>/track', methods=['POST'])
def track_task_time(task_id):
    data = request.get_json()
    user_id = data.get('user_id')
    
    # 接收前端发送的三个状态数据
    new_time_spent = data.get('time_spent')       # 累计总秒数
    timer_start_time = data.get('timer_start_time') # 开始时间戳 (Float)
    
    task = Task.query.filter_by(id=task_id, user_id=user_id).first()
    if not task:
        return jsonify({'error': '任务不存在或不属于该用户'}), 404

    # 1. 更新累计总时间 (用于暂停或离线时同步)
    if new_time_spent is not None:
        task.time_spent_seconds += new_time_spent

    # 2. 更新计时器开始时间
    # 无论是 None (暂停) 还是一个时间戳 (开始/继续) 
    task.timer_start_time = timer_start_time
    
    db.session.commit()

    return jsonify({
        'message': '计时数据已同步', 
        'time_spent_seconds': task.time_spent_seconds,
        'timer_start_time': task.timer_start_time
    }), 200



# app.py
#每周更新任务列表
@app.route('/api/tasks/settlement', methods=['POST'])
def settle_task():
    data = request.get_json()
    user_id = data.get('user_id')
    season_weak=data.get('season_week')
    tasks = Task.query.filter_by(user_id=user_id, status='completed').all()
    user = User.query.filter_by(id=user_id).first()
    total_points,total_time=0,0
    task_lists=[]
    if not tasks:
        user.last_settlement_date = datetime.utcnow()
        db.session.commit()
        return jsonify({'message': '没有已完成的任务需要结算'}), 200
    for task in tasks:
        total_time+=task.time_spent_seconds
        task_lists.append(task.id)
        task.status='archived'
        total_points+=task.points

    total_points=math.floor(total_points*10)/10
    task_count = len(tasks)

    # 创建每周结算记录
    new_record = WeekRecord(
        user_id=user_id,
        name=season_weak,
        total_points=total_points,
        total_num=task_count,
        task_lists=task_lists,
        total_time=total_time
    )
    db.session.add(new_record)

    user.last_settlement_date = datetime.utcnow()
    db.session.commit()

    ave_daytime=math.floor(total_time/7)
    return jsonify({'message':f'😻本周共完成 {task_count} 项任务，获得 {total_points} 积分。总共花费 {math.floor(total_time/3600)}小时{math.floor((total_time%3600)/60)}分钟{total_time%60}秒。每天平均花费{math.floor(ave_daytime/3600)}小时{math.floor((ave_daytime%3600)/60)}分钟{ave_daytime%60}秒。已完成的任务已被归档，让我们迎接新的一周，制定新的目标和计划吧！'
                    }), 200


#获取上次周更新时间
@app.route('/api/<string:user_id>/lastSettlementDate', methods=['GET'])
def last_settlement_date(user_id):
    user=User.query.filter_by(id=user_id).first()
    return jsonify({'last_settlement_date': user.last_settlement_date.isoformat() + 'Z'})

# ... 其他路由 ...

# 获取公共商品 API
@app.route('/api/shop/public', methods=['GET'])
def get_public_shop_items():
    items = ShopItem.query.filter_by(type='public').order_by(ShopItem.created_at.desc()).all()
    items_list = [{
        'id': item.id,
        'name': item.name,
        'points': item.points,
        'image': item.image,
        'created_at': item.created_at.strftime('%Y-%m-%d') 
    } for item in items]
    return jsonify(items_list), 200

# 获取自定义商品 API
@app.route('/api/shop/private/<string:user_id>', methods=['GET'])
def get_private_shop_items(user_id):
    items = ShopItem.query.filter_by(user_id=user_id, type='private').order_by(ShopItem.created_at.desc()).all()
    items_list = [{
        'id': item.id,
        'name': item.name,
        'points': item.points,
        'image': item.image,
        'created_at': item.created_at.strftime('%Y-%m-%d') 
    } for item in items]
    return jsonify(items_list), 200

# 创建自定义商品 API
@app.route('/api/shop/private', methods=['POST'])
def create_private_shop_item():
    data = request.get_json()
    user_id = data.get('user_id')
    name = data.get('name')
    points = data.get('points')
    type = data.get('type')
    image = data.get('image')
    rate = data.get('rate')

    user=User.query.filter_by(id=user_id).first()
    points/=rate
    points=math.floor(points*10)/10

    if not all([user_id, name, points, type, image, rate]):
        return jsonify({'error': '缺少必要的商品信息'}), 400
    
    user.exchange_rate=rate

    new_item = ShopItem(
        user_id=user_id,
        name=name,
        points=points,
        image=image,
        type=type
    )
    db.session.add(new_item)
    db.session.commit()
    
    return jsonify({
        'message': '自定义商品创建成功',
        'rate': rate
    }), 201


# 兑换商品 API
@app.route('/api/shop/redeem', methods=['POST'])
def redeem_item():
    data = request.get_json()
    user_id = data.get('user_id')
    item_id = data.get('item_id')

    user = User.query.filter_by(id=user_id).first()
    if not user:
        return jsonify({'error': '用户不存在'}), 404
    
    item = ShopItem.query.filter_by(id=item_id).first()
    if not item:
        return jsonify({'error': '商品不存在'}), 404

    if user.current_points < item.points:
        return jsonify({'error': '积分不足'}), 402 # 402 Payment Required
    
    # 扣除积分并创建兑换记录
    user.current_points -= item.points
    user.current_points=math.floor(user.current_points*10)/10
    new_redeemed = RedeemedItem(
        user_id=user_id,
        item_name=item.name,
        item_points=item.points,
        item_image=item.image
    )
    if item.type=="private":
        db.session.delete(item)

    db.session.add(new_redeemed)
    db.session.commit()
    
    return jsonify({
        'message': '兑换成功',
        'current_points':f'{user.current_points}'
    }), 200

@app.route('/api/shop/info', methods=['POST'])
def shop_info():
    data = request.get_json()
    user_id = data.get('user_id')
    item_id = data.get('item_id')
    user = User.query.filter_by(id=user_id).first()
    if not user:
        return jsonify({'error': '用户不存在'}), 404
    
    item = ShopItem.query.filter_by(id=item_id).first()
    if not item:
        return jsonify({'error': '商品不存在'}), 404
    
    return jsonify({
        'type':item.type
    }), 200

# 修改商品 API
@app.route('/api/shop/alter', methods=['POST'])
def alter_item():
    data = request.get_json()
    user_id = data.get('user_id')
    item_id = data.get('item_id')
    newPoints=data.get('item_points')

    user = User.query.filter_by(id=user_id).first()
    if not user:
        return jsonify({'error': '用户不存在'}), 404
    
    newPoints/=user.exchange_rate
    newPoints=math.floor(newPoints*10)/10
    
    item = ShopItem.query.filter_by(id=item_id).first()
    if not item:
        return jsonify({'error': '商品不存在'}), 404
    

    item.points=newPoints

    db.session.commit()

    return jsonify({
        'message': '兑换成功',
        'current_points':f'{user.current_points}',
        'name':item.name,
        'type':item.type
    }), 200

#删除商品 API
@app.route('/api/shop/delete', methods=['DELETE'])
def delete_item():
    data = request.get_json()
    user_id = data.get('user_id')
    item_id = data.get('item_id')

    item=ShopItem.query.filter_by(id=item_id).first()
    if not item:
        return jsonify({'error': '商品不存在'}), 404
    
    user = User.query.filter_by(id=user_id).first()

    count_query = db.session.execute(
        db.select(db.func.count())
        .select_from(RedeemedItem)
        .filter_by(item_image=item.image)
    ).scalar_one()

    if count_query == 0:
        error=delete_file(item.image)
        if 'error' in error:
            return jsonify({'error': error[error]}), 402
    
    db.session.delete(item)
    db.session.commit()

    return jsonify({'message': '商品已成功删除',
        'current_points':f'{user.current_points}',
        'name':item.name}), 200


# 获取用户信息和兑换记录 API
@app.route('/api/user/<string:user_id>', methods=['GET'])
def get_user_info(user_id):
    user = User.query.filter_by(id=user_id).first()
    if not user:
        return jsonify({'error': '用户不存在'}), 404
    
    redeemed_items = RedeemedItem.query.filter_by(user_id=user_id).order_by(RedeemedItem.redeemed_at.desc()).all()
    redeemed_list = [{
        'name': item.item_name,
        'points': item.item_points,
        'image': item.item_image,
        'redeemed_at': item.redeemed_at.strftime('%Y-%m-%d') 
    } for item in redeemed_items]
    
    user_info = {
        'nickname': user.nickname,
        'avatar': user.avatar,
        'current_points': user.current_points,
        'total_earned_points': user.total_earned_points,
        'redeemed_items': redeemed_list,
        'rate': user.exchange_rate
    }
    
    return jsonify(user_info), 200


ALLOWED_EXTENSIONS = {'png', 'jpg', 'jpeg', 'gif', 'webp', 'heic', 'heif'}

def allowed_file(filename):
    return '.' in filename and filename.rsplit('.', 1)[1].lower() in ALLOWED_EXTENSIONS


# 文件上传 API
@app.route('/api/upload', methods=['POST'])
def upload_file():

    # 检查请求中是否包含文件
    if 'file' not in request.files:
        return jsonify({'error': '未找到文件'}), 400

    file = request.files['file']

    # 检查文件名是否为空
    if not allowed_file(file.filename):
        return jsonify({'error': '文件名为空'}), 400

    if file:
        # 确保文件名安全，并使用UUID生成唯一的文件名
        filename = secure_filename(file.filename)
        file_extension = os.path.splitext(filename)[1]
        unique_filename = str(uuid.uuid4()) + file_extension
        
        file_path = os.path.join(app.config['UPLOAD_FOLDER'], unique_filename)
        file.save(file_path)
        
        # 返回文件的公共访问URL
        image_url = f'/uploads/{unique_filename}'
        return jsonify({'message': '文件上传成功', 'url': image_url}), 201

    
# 假设 app, UPLOAD_FOLDER, jsonify, request 已经定义和导入
# 请确保你的 UPLOAD_FOLDER 路径在 app.config 中正确配置

def delete_file(file_url):

    if not file_url:
        return jsonify({'error': '请提供文件 URL'}), 400

    # 2. 从 URL 中提取文件名
    # 假设 URL 格式是 /uploads/unique_filename.ext
    parsed_url = urlparse(file_url)
    # path 是 /uploads/unique_filename.ext
    # os.path.basename() 提取路径中的文件名部分
    unique_filename = os.path.basename(parsed_url.path)

    if not unique_filename:
        return {'error': 'URL格式无效'}

    # 3. 构造文件的完整路径
    upload_folder = app.config['UPLOAD_FOLDER'] # 使用 current_app 获取配置
    file_path = os.path.join(upload_folder, unique_filename)

    # 4. 检查文件是否存在并删除
    if os.path.exists(file_path):
        try:
            # 5. 执行删除操作
            os.remove(file_path)
            return {
                'message': '文件删除成功', 
                'filename': unique_filename
            }
        except OSError as e:
            # 处理权限或其他操作系统错误
            print(f"删除文件时发生错误: {e}")
            return {'error': '删除文件失败，请检查权限'}
    else:
        # 文件不存在
        return {'message': '文件未找到或已被删除'}

# 提供静态文件的路由
@app.route('/uploads/<filename>')
def uploaded_file(filename):
    return send_from_directory(app.config['UPLOAD_FOLDER'], filename)

# app.py

# ... 在其他路由后面添加 ...

#更新用户昵称 API
@app.route('/api/user/nickname', methods=['POST'])
def update_nickname():
    data = request.get_json()
    user_id = data.get('user_id')
    nickname=data.get('newNickname')

    user = User.query.filter_by(id=user_id).first()
    if not user:
        return jsonify({'error': '用户不存在'}), 404
    
    user.nickname = nickname
    db.session.commit()

    return jsonify({'message': '昵称更新成功'}), 200


# 更新用户头像 API
@app.route('/api/user/update_avatar', methods=['POST'])
def update_avatar():
    data = request.get_json()
    user_id = data.get('user_id')
    avatar_url = data.get('avatar_url')

    user = User.query.filter_by(id=user_id).first()
    if not user:
        return jsonify({'error': '用户不存在'}), 404

    error=delete_file(user.avatar)
    if 'error' in error and error[error]!='文件未找到或已被删除':
        return jsonify({'error': '头像文件删除失败'}), 404
    
    user.avatar = avatar_url
    db.session.commit()
    
    return jsonify({'message': '头像更新成功'}), 200

if __name__ == '__main__':
    app.run(debug=True)