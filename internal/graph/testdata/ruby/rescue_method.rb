class UserService
  def find_user(id)
    begin
      user = User.find(id)
      user.update(last_seen: Time.now)
    rescue ActiveRecord::RecordNotFound
      render plain: "Not found", status: 404
    ensure
      connection.release
    end
  end
end
